package polybft

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/bitmap"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/signer"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/state"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/Hydra-Chain/go-ibft/messages"
	"github.com/Hydra-Chain/go-ibft/messages/proto"
	"github.com/armon/go-metrics"
	hcf "github.com/hashicorp/go-hclog"
)

type blockBuilder interface {
	Reset() error
	WriteTx(*types.Transaction) error
	Fill()
	Build(func(h *types.Header)) (*types.FullBlock, error)
	GetState() *state.Transition
	Receipts() []*types.Receipt
}

var (
	errCommitEpochTxDoesNotExist = errors.New(
		"commit epoch transaction is not found in the epoch ending block",
	)
	errCommitEpochTxNotExpected = errors.New(
		"didn't expect commit epoch transaction in a non epoch ending block",
	)
	errCommitEpochTxSingleExpected = errors.New("only one commit epoch transaction is allowed " +
		"in an epoch ending block")
	errFundRewardWalletTxDoesNotExists = errors.New("fund reward wallet transaction is " +
		"not found in the epoch ending block")
	errFundRewardWalletTxSingleExpected = errors.New(
		"only one fund reward wallet transaction is allowed " +
			"in an epoch ending block",
	)
	errDistributeRewardsTxDoesNotExist = errors.New("distribute rewards transaction is " +
		"not found in the epoch ending block")
	errDistributeRewardsTxNotExpected = errors.New("didn't expect distribute rewards transaction " +
		"in a non epoch ending block")
	errDistributeRewardsTxSingleExpected = errors.New(
		"only one distribute rewards transaction is " +
			"allowed in an epoch ending block",
	)
	errDistributeDAOIncentiveTxDoesNotExist = errors.New(
		"distribute DAO incentive transaction is " +
			"not found in the epoch ending block",
	)
	errDistributeDAOIncentiveTxNotExpected = errors.New(
		"didn't expect distribute DAO incentive transaction " +
			"in a non epoch ending block",
	)
	errDistributeDAOIncentiveTxSingleExpected = errors.New(
		"only one distribute DAO incentive transaction is " +
			"allowed in an epoch ending block",
	)
	errSyncValidatorsDataTxDoesNotExist = errors.New(
		"sync validators data transaction is not found in the epoch starting block",
	)
	errSyncValidatorsDataTxSingleExpected = errors.New(
		"only one sync validators data transaction is allowed " +
			"in an epoch starting block",
	)
	errSyncValidatorsDataTxNotExpected = errors.New(
		"didn't expect sync validators data transaction " +
			"in a non epoch starting block",
	)
	errProposalDontMatch = errors.New("failed to insert proposal, because the validated proposal " +
		"is either nil or it does not match the received one")
	errValidatorSetDeltaMismatch        = errors.New("validator set delta mismatch")
	errValidatorsUpdateInNonEpochEnding = errors.New(
		"trying to update validator set in a non epoch ending block",
	)
	errValidatorDeltaNilInEpochEndingBlock = errors.New(
		"validator set delta is nil in epoch ending block",
	)
	errCommitEpochTxRequired = errors.New(
		"the commit epoch transaction must be executed first",
	)
	errFundRewardWalletTxRequired = errors.New(
		"the reward wallet fund transaction must be executed before distributing rewards",
	)
)

type fsm struct {
	// PolyBFT consensus protocol configuration
	config *PolyBFTConfig

	// parent block header
	parent *types.Header

	// backend implements methods for retrieving data from block chain
	backend BlockchainBackend

	// polybftBackend implements methods needed from the polybft
	polybftBackend polybftBackend

	// validators is the list of validators for this round
	validators validator.ValidatorSet

	// proposerSnapshot keeps information about new proposer
	proposerSnapshot *ProposerSnapshot

	// blockBuilder is the block builder for proposers
	blockBuilder blockBuilder

	// epochNumber denotes current epoch number
	epochNumber uint64

	// commitEpochInput holds info about a single epoch
	// It is populated only for epoch-ending blocks.
	commitEpochInput *contractsapi.CommitEpochHydraChainFn

	// distributeRewardsInput holds info about validators work in a single epoch
	// mainly, how many blocks they signed during given epoch
	// It is populated only for epoch-ending blocks.
	distributeRewardsInput *contractsapi.DistributeRewardsForHydraStakingFn

	// fund the RewardWallet which is executed before each distributeRewardsFor
	// in order to keep enough funds in the contract
	fundRewardWalletInput *contractsapi.FundRewardWalletFn

	// rewardWalletFundAmount holds the value of the HYDRA amount that needs to fulfill the reward wallet
	// It is send to the RewardWallet contract on fund transaction
	// It is populated only for epoch-ending blocks when there are no sufficient funds.
	rewardWalletFundAmount *big.Int

	// distributeDAOIncentiveInputs will be used to distribute DAO incentive at the end of each epoch
	distributeDAOIncentiveInput *contractsapi.DistributeDAOIncentiveHydraChainFn

	// syncValidatorsDataInput holds info about the updated validators' voting power
	// It is populated only for epoch-starting blocks.
	syncValidatorsDataInput *contractsapi.SyncValidatorsDataHydraChainFn

	// isEndOfEpoch indicates if epoch reached its end
	isEndOfEpoch bool

	// isEndOfSprint indicates if sprint reached its end
	isEndOfSprint bool

	// isStartOfEpoch indicates if epoch has started in the current block
	isStartOfEpoch bool

	// proposerCommitmentToRegister is a commitment that is registered via state transaction by proposer
	proposerCommitmentToRegister *CommitmentMessageSigned

	// logger instance
	logger hcf.Logger

	// target is the block being computed
	target *types.FullBlock

	// exitEventRootHash is the calculated root hash for given checkpoint block
	exitEventRootHash types.Hash

	// newValidatorsDelta carries the updates of validator set on epoch ending block
	newValidatorsDelta *validator.ValidatorSetDelta
}

// BuildProposal builds a proposal for the current round (used if proposer)
func (f *fsm) BuildProposal(currentRound uint64) ([]byte, error) {
	start := time.Now().UTC()
	defer metrics.SetGauge([]string{consensusMetricsPrefix, "block_building_time"},
		float32(time.Now().UTC().Sub(start).Seconds()))

	parent := f.parent

	extraParent, err := GetIbftExtra(parent.ExtraData)
	if err != nil {
		return nil, err
	}

	extra := &Extra{Parent: extraParent.Committed}
	// for non-epoch ending blocks, currentValidatorsHash is the same as the nextValidatorsHash
	nextValidators := f.validators.Accounts()

	if err := f.blockBuilder.Reset(); err != nil {
		return nil, fmt.Errorf("failed to initialize block builder: %w", err)
	}

	if f.isEndOfEpoch {
		tx, err := f.createCommitEpochTx()
		if err != nil {
			return nil, err
		}

		if err := f.blockBuilder.WriteTx(tx); err != nil {
			return nil, fmt.Errorf("failed to apply commit epoch transaction: %w", err)
		}

		// if fund amount is 0, then no need to fund the reward wallet
		if f.rewardWalletFundAmount != big.NewInt(0) {
			tx, err = f.createRewardWalletFundTx()
			if err != nil {
				return nil, err
			}

			if err := f.blockBuilder.WriteTx(tx); err != nil {
				return nil, fmt.Errorf(
					"failed to apply the RewardWallet contract fund transaction: %w",
					err,
				)
			}
		}

		tx, err = f.createDistributeRewardsTx()
		if err != nil {
			return nil, err
		}

		if err := f.blockBuilder.WriteTx(tx); err != nil {
			return nil, fmt.Errorf("failed to apply distribute rewards transaction: %w", err)
		}

		tx, err = f.createDistributeDAOIncentiveTx()
		if err != nil {
			return nil, err
		}

		if err := f.blockBuilder.WriteTx(tx); err != nil {
			return nil, fmt.Errorf(
				"failed to apply distribute DAO incentive rewards transaction: %w",
				err,
			)
		}
	} else if f.isStartOfEpoch && f.syncValidatorsDataInput != nil {
		tx, err := f.createSyncValidatorsDataTx()
		if err != nil {
			return nil, err
		}

		if err := f.blockBuilder.WriteTx(tx); err != nil {
			return nil, fmt.Errorf(
				"failed to apply sync validators data transaction: %w",
				err,
			)
		}
	}

	if f.config.IsBridgeEnabled() {
		if err := f.applyBridgeCommitmentTx(); err != nil {
			return nil, err
		}
	}

	// fill the block with transactions
	f.blockBuilder.Fill()

	if f.isEndOfEpoch {
		nextValidators, err = nextValidators.ApplyDelta(f.newValidatorsDelta)
		if err != nil {
			return nil, err
		}

		extra.Validators = f.newValidatorsDelta
	}

	currentValidatorsHash, err := f.validators.Accounts().Hash()
	if err != nil {
		return nil, err
	}

	nextValidatorsHash, err := nextValidators.Hash()
	if err != nil {
		return nil, err
	}

	extra.Checkpoint = &CheckpointData{
		BlockRound:            currentRound,
		EpochNumber:           f.epochNumber,
		CurrentValidatorsHash: currentValidatorsHash,
		NextValidatorsHash:    nextValidatorsHash,
		EventRoot:             f.exitEventRootHash,
	}

	f.logger.Debug("[Build Proposal]", "Current validators hash", currentValidatorsHash,
		"Next validators hash", nextValidatorsHash)

	stateBlock, err := f.blockBuilder.Build(func(h *types.Header) {
		h.ExtraData = extra.MarshalRLPTo(nil)
		h.MixHash = HydragonMixDigest
	})

	if err != nil {
		return nil, err
	}

	if f.logger.IsDebug() {
		checkpointHash, err := extra.Checkpoint.Hash(
			f.backend.GetChainID(),
			f.Height(),
			stateBlock.Block.Hash(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate proposal hash: %w", err)
		}

		f.logger.Debug("[FSM Build Proposal]",
			"txs", len(stateBlock.Block.Transactions),
			"proposal hash", checkpointHash.String())
	}

	f.target = stateBlock

	return stateBlock.Block.MarshalRLP(), nil
}

// applyBridgeCommitmentTx builds state transaction which contains data for bridge commitment registration
func (f *fsm) applyBridgeCommitmentTx() error {
	if f.proposerCommitmentToRegister != nil {
		bridgeCommitmentTx, err := f.createBridgeCommitmentTx()
		if err != nil {
			return fmt.Errorf("creation of bridge commitment transaction failed: %w", err)
		}

		if err := f.blockBuilder.WriteTx(bridgeCommitmentTx); err != nil {
			return fmt.Errorf("failed to apply bridge commitment state transaction. Error: %w", err)
		}
	}

	return nil
}

// createBridgeCommitmentTx builds bridge commitment registration transaction
func (f *fsm) createBridgeCommitmentTx() (*types.Transaction, error) {
	inputData, err := f.proposerCommitmentToRegister.EncodeAbi()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to encode input data for bridge commitment registration: %w",
			err,
		)
	}

	return createStateTransactionWithData(
		f.Height(),
		contracts.StateReceiverContract,
		inputData,
		nil,
	), nil
}

// getValidatorsTransition applies delta to the current validators,
func (f *fsm) getValidatorsTransition(
	delta *validator.ValidatorSetDelta,
) (validator.AccountSet, error) {
	nextValidators, err := f.validators.Accounts().ApplyDelta(delta)
	if err != nil {
		return nil, err
	}

	f.logger.Debug("getValidatorsTransition", "Next validators", nextValidators)

	return nextValidators, nil
}

// createCommitEpochTx create a StateTransaction, which invokes ValidatorSet smart contract
// and sends all the necessary metadata to it.
func (f *fsm) createCommitEpochTx() (*types.Transaction, error) {
	input, err := f.commitEpochInput.EncodeAbi()
	if err != nil {
		return nil, err
	}

	return createStateTransactionWithData(f.Height(), contracts.HydraChainContract, input, nil), nil
}

// createDistributeRewardsTx create a StateTransaction, which invokes HydraStaking smart contract
// and sends all the necessary metadata to it.
func (f *fsm) createDistributeRewardsTx() (*types.Transaction, error) {
	input, err := f.distributeRewardsInput.EncodeAbi()
	if err != nil {
		return nil, err
	}

	return createStateTransactionWithData(
		f.Height(),
		contracts.HydraStakingContract,
		input,
		nil,
	), nil
}

// createRewardWalletFundTx create a StateTransaction, which invokes the RewardWallet smart contract
// and sends some funds to it.
func (f *fsm) createRewardWalletFundTx() (*types.Transaction, error) {
	input, err := f.fundRewardWalletInput.EncodeAbi()
	if err != nil {
		return nil, err
	}

	return createStateTransactionWithData(
		f.Height(),
		contracts.RewardWalletContract,
		input,
		f.rewardWalletFundAmount,
	), nil
}

// createDistributeDAOIncentiveTx create a StateTransaction, which invokes HydraChain smart contract
// and sends all the necessary metadata to it.
func (f *fsm) createDistributeDAOIncentiveTx() (*types.Transaction, error) {
	input, err := f.distributeDAOIncentiveInput.EncodeAbi()
	if err != nil {
		return nil, err
	}

	return createStateTransactionWithData(
		f.Height(),
		contracts.HydraChainContract,
		input,
		nil,
	), nil
}

// createSyncValidatorsDataTx create a StateTransaction, which invokes HydraChain smart contract
// and sends all the necessary metadata to it.
func (f *fsm) createSyncValidatorsDataTx() (*types.Transaction, error) {
	input, err := f.syncValidatorsDataInput.EncodeAbi()
	if err != nil {
		return nil, err
	}

	return createStateTransactionWithData(
		f.Height(),
		contracts.HydraChainContract,
		input,
		nil,
	), nil
}

// ValidateCommit is used to validate that a given commit is valid
func (f *fsm) ValidateCommit(signerAddr []byte, seal []byte, proposalHash []byte) error {
	from := types.BytesToAddress(signerAddr)

	validator := f.validators.Accounts().GetValidatorMetadata(from)
	if validator == nil {
		return fmt.Errorf("unable to resolve validator %s", from)
	}

	signature, err := bls.UnmarshalSignature(seal)
	if err != nil {
		return fmt.Errorf("failed to unmarshall signature: %w", err)
	}

	if !signature.Verify(validator.BlsKey, proposalHash, signer.DomainCheckpointManager) {
		return fmt.Errorf("incorrect commit signature from %s", from)
	}

	return nil
}

// Validate validates a raw proposal (used if non-proposer)
func (f *fsm) Validate(proposal []byte) error {
	var block types.Block
	if err := block.UnmarshalRLP(proposal); err != nil {
		return fmt.Errorf("failed to validate, cannot decode block data. Error: %w", err)
	}

	// validate header fields
	if err := validateHeaderFields(f.parent, block.Header, f.config.BlockTimeDrift); err != nil {
		return fmt.Errorf(
			"failed to validate header (parent header# %d, current header#%d): %w",
			f.parent.Number,
			block.Number(),
			err,
		)
	}

	extra, err := GetIbftExtra(block.Header.ExtraData)
	if err != nil {
		return fmt.Errorf("cannot get extra data:%w", err)
	}

	parentExtra, err := GetIbftExtra(f.parent.ExtraData)
	if err != nil {
		return err
	}

	if extra.Checkpoint == nil {
		return fmt.Errorf("checkpoint data for block %d is missing", block.Number())
	}

	if parentExtra.Checkpoint == nil {
		return fmt.Errorf("checkpoint data for parent block %d is missing", f.parent.Number)
	}

	if err := extra.ValidateParentSignatures(block.Number(), f.polybftBackend, nil, f.parent, parentExtra,
		f.backend.GetChainID(), signer.DomainCheckpointManager, f.logger); err != nil {
		return err
	}

	if err := f.VerifyStateTransactions(block.Transactions); err != nil {
		return err
	}

	currentValidators := f.validators.Accounts()

	// validate validators delta
	if f.isEndOfEpoch {
		if extra.Validators == nil {
			return errValidatorDeltaNilInEpochEndingBlock
		}

		if !extra.Validators.Equals(f.newValidatorsDelta) {
			return errValidatorSetDeltaMismatch
		}
	} else if extra.Validators != nil {
		// delta should be nil in non epoch ending blocks
		return errValidatorsUpdateInNonEpochEnding
	}

	nextValidators, err := f.getValidatorsTransition(extra.Validators)
	if err != nil {
		return err
	}

	// validate checkpoint data
	if err := extra.Checkpoint.Validate(parentExtra.Checkpoint,
		currentValidators, nextValidators, f.exitEventRootHash); err != nil {
		return err
	}

	if f.logger.IsTrace() && block.Number() > 1 {
		validators, err := f.polybftBackend.GetValidators(block.Number()-2, nil)
		if err != nil {
			return fmt.Errorf("failed to retrieve validators:%w", err)
		}

		f.logger.Trace("[FSM Validate]", "Block", block.Number(), "parent validators", validators)
	}

	stateBlock, err := f.backend.ProcessBlock(f.parent, &block)
	if err != nil {
		return err
	}

	if f.logger.IsDebug() {
		checkpointHash, err := extra.Checkpoint.Hash(
			f.backend.GetChainID(),
			block.Number(),
			block.Hash(),
		)
		if err != nil {
			return fmt.Errorf("failed to calculate proposal hash: %w", err)
		}

		f.logger.Debug(
			"[FSM Validate]",
			"txs",
			len(block.Transactions),
			"proposal hash",
			checkpointHash,
		)
	}

	f.target = stateBlock

	return nil
}

// ValidateSender validates sender address and signature
func (f *fsm) ValidateSender(msg *proto.Message) error {
	msgNoSig, err := msg.PayloadNoSig()
	if err != nil {
		return err
	}

	signerAddress, err := wallet.RecoverAddressFromSignature(msg.Signature, msgNoSig)
	if err != nil {
		return fmt.Errorf("failed to recover address from signature: %w", err)
	}

	// verify the signature came from the sender
	if !bytes.Equal(msg.From, signerAddress.Bytes()) {
		return fmt.Errorf("signer address %s doesn't match From field", signerAddress.String())
	}

	// verify the sender is in the active validator set
	if !f.validators.Includes(signerAddress) {
		return fmt.Errorf(
			"signer address %s is not included in validator set",
			signerAddress.String(),
		)
	}

	return nil
}

func (f *fsm) VerifyStateTransactions(transactions []*types.Transaction) error {
	var (
		commitEpochTxExists            bool
		fundRewardWalletTxExists       bool
		distributeRewardsTxExists      bool
		distributeDAOIncentiveTxExists bool
		syncValidatorsDataTxExists     bool
	)

	for i, tx := range transactions {
		if tx.Type != types.StateTx {
			continue
		}

		decodedStateTx, err := decodeStateTransaction(tx.Input)
		if err != nil {
			return fmt.Errorf("unknown state transaction: tx = %v, err = %w", tx.Hash, err)
		}

		switch stateTxData := decodedStateTx.(type) {
		case *contractsapi.CommitEpochHydraChainFn:
			if commitEpochTxExists {
				// if we already validated commit epoch tx,
				// that means someone added more than one commit epoch tx to block,
				// which is invalid
				return errCommitEpochTxSingleExpected
			}

			expectedIndex := 0
			if i != expectedIndex {
				return InvalidTxIndexErr(expectedIndex, i)
			}

			commitEpochTxExists = true

			if err := f.verifyCommitEpochTx(tx); err != nil {
				return fmt.Errorf("error while verifying commit epoch transaction. error: %w", err)
			}
		case *contractsapi.FundRewardWalletFn:
			if fundRewardWalletTxExists {
				// if we already validated fund reward wallet tx,
				// that means someone added more txs to fund reward wallet,
				// which is invalid
				return errFundRewardWalletTxSingleExpected
			}

			fundRewardWalletTxExists = true

			if err := f.verifyRewardWalletFundTx(tx); err != nil {
				return fmt.Errorf("error while verifying fund reward wallet transaction. error: %w", err)
			}
		case *contractsapi.DistributeRewardsForHydraStakingFn:
			if distributeRewardsTxExists {
				// if we already validated distribute rewards tx,
				// that means someone added more than one distribute rewards tx to block,
				// which is invalid
				return errDistributeRewardsTxSingleExpected
			}

			if f.isRewardWalletFundTxRequired() && !fundRewardWalletTxExists {
				return errFundRewardWalletTxRequired
			}

			distributeRewardsTxExists = true

			if err := f.verifyDistributeRewardsTx(tx); err != nil {
				return fmt.Errorf("error while verifying distribute rewards transaction. error: %w", err)
			}
		case *contractsapi.DistributeDAOIncentiveHydraChainFn:
			if distributeDAOIncentiveTxExists {
				// if we already validated distribute DAO incentive tx,
				// that means someone added more than one distribute DAO incentive tx to block,
				// which is invalid
				return errDistributeDAOIncentiveTxSingleExpected
			}

			if f.isRewardWalletFundTxRequired() && !fundRewardWalletTxExists {
				return errFundRewardWalletTxRequired
			}

			distributeDAOIncentiveTxExists = true

			if err := f.verifyDistributeDAOIncentiveTx(tx); err != nil {
				return fmt.Errorf("error while verifying distribute DAO incentive rewards transaction. error: %w", err)
			}
		case *contractsapi.SyncValidatorsDataHydraChainFn:
			if syncValidatorsDataTxExists {
				// if we already validated sync validators data tx,
				// that means someone added more than one sync validators data tx to block,
				// which is invalid
				return errSyncValidatorsDataTxSingleExpected
			}

			syncValidatorsDataTxExists = true

			if err := f.verifySyncValidatorsDataTx(tx, i); err != nil {
				return fmt.Errorf("error while verifying sync validators data transaction. error: %w", err)
			}
		default:
			return fmt.Errorf("invalid state transaction data type: %v", stateTxData)
		}
	}

	if f.isEndOfEpoch {
		if !commitEpochTxExists {
			// this is a check if commit epoch transaction is not in the list of transactions at all
			// but it should be
			return errCommitEpochTxDoesNotExist
		}

		if f.isRewardWalletFundTxRequired() && !fundRewardWalletTxExists {
			// this is a check if there is a need to fund the reward wallet, but the transaction is not in the
			// list of transactions at all, but it should be
			return errFundRewardWalletTxDoesNotExists
		}

		if !distributeRewardsTxExists {
			// this is a check if distribute rewards transaction is not in the list of transactions at all
			// but it should be
			return errDistributeRewardsTxDoesNotExist
		}

		if !distributeDAOIncentiveTxExists {
			// this is a check if distribute DAO incentive rewards tx is not in the list of transactions at all
			// but it should be
			return errDistributeDAOIncentiveTxDoesNotExist
		}
	}

	if f.isStartOfEpoch {
		if f.isSyncValidatorsDataTxRequired() && !syncValidatorsDataTxExists {
			// this is a check if sync validators data transaction is not in the list of transactions
			// at all, but it should be
			return errSyncValidatorsDataTxDoesNotExist
		}
	}

	return nil
}

// Insert inserts the sealed proposal
func (f *fsm) Insert(
	proposal []byte,
	committedSeals []*messages.CommittedSeal,
) (*types.FullBlock, error) {
	newBlock := f.target

	var proposedBlock types.Block
	if err := proposedBlock.UnmarshalRLP(proposal); err != nil {
		return nil, fmt.Errorf("failed to insert proposal, block unmarshaling failed: %w", err)
	}

	if newBlock == nil || newBlock.Block.Hash() != proposedBlock.Hash() {
		// if this is the case, we will let syncer insert the block
		return nil, errProposalDontMatch
	}

	// In this function we should try to return little to no errors since
	// at this point everything we have to do is just commit something that
	// we should have already computed beforehand.
	extra, err := GetIbftExtra(newBlock.Block.Header.ExtraData)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to insert proposal, due to not being able to extract extra data: %w",
			err,
		)
	}

	// create map for faster access to indexes
	nodeIDIndexMap := make(map[types.Address]int, f.validators.Len())
	for i, addr := range f.validators.Accounts().GetAddresses() {
		nodeIDIndexMap[addr] = i
	}

	// populated bitmap according to nodeId from validator set and committed seals
	// also populate slice of signatures
	bitmap := bitmap.Bitmap{}
	signatures := make(bls.Signatures, 0, len(committedSeals))

	for _, commSeal := range committedSeals {
		signerAddr := types.BytesToAddress(commSeal.Signer)

		index, exists := nodeIDIndexMap[signerAddr]
		if !exists {
			return nil, fmt.Errorf("invalid node id = %s", signerAddr.String())
		}

		s, err := bls.UnmarshalSignature(commSeal.Signature)
		if err != nil {
			return nil, fmt.Errorf("invalid signature = %s", commSeal.Signature)
		}

		signatures = append(signatures, s)

		bitmap.Set(uint64(index))
	}

	aggregatedSignature, err := signatures.Aggregate().Marshal()
	if err != nil {
		return nil, fmt.Errorf("could not aggregate seals: %w", err)
	}

	// include aggregated signature of all committed seals
	// also includes bitmap which contains all indexes from validator set which provides there seals
	extra.Committed = &Signature{
		AggregatedSignature: aggregatedSignature,
		Bitmap:              bitmap,
	}

	// Write extra data to header
	newBlock.Block.Header.ExtraData = extra.MarshalRLPTo(nil)

	if err := f.backend.CommitBlock(newBlock); err != nil {
		return nil, err
	}

	return newBlock, nil
}

// Height returns the height for the current round
func (f *fsm) Height() uint64 {
	return f.parent.Number + 1
}

// ValidatorSet returns the validator set for the current round
func (f *fsm) ValidatorSet() validator.ValidatorSet {
	return f.validators
}

// verifyCommitEpochTx creates commit epoch transaction and compares its hash with the one extracted from the block.
func (f *fsm) verifyCommitEpochTx(commitEpochTx *types.Transaction) error {
	if f.isEndOfEpoch {
		localCommitEpochTx, err := f.createCommitEpochTx()
		if err != nil {
			return err
		}

		if commitEpochTx.Hash != localCommitEpochTx.Hash {
			return fmt.Errorf(
				"invalid commit epoch transaction. Expected '%s', but got '%s' commit epoch transaction hash",
				localCommitEpochTx.Hash,
				commitEpochTx.Hash,
			)
		}

		return nil
	}

	return errCommitEpochTxNotExpected
}

// verifyCommitEpochTx creates commit epoch transaction and compares its hash with the one extracted from the block.
func (f *fsm) verifyRewardWalletFundTx(fundRewardWalletTx *types.Transaction) error {
	if f.isEndOfEpoch {
		localFundRewardWalletTx, err := f.createRewardWalletFundTx()
		if err != nil {
			return err
		}

		if fundRewardWalletTx.Hash != localFundRewardWalletTx.Hash {
			return fmt.Errorf(
				"invalid fund reward wallet transaction. Expected '%s', but got '%s' fund reward wallet transaction hash",
				localFundRewardWalletTx.Hash,
				fundRewardWalletTx.Hash,
			)
		}

		return nil
	}

	return errFundRewardWalletTxSingleExpected
}

// verifyDistributeRewardsTx creates distribute rewards transaction
// and compares its hash with the one extracted from the block.
func (f *fsm) verifyDistributeRewardsTx(distributeRewardsTx *types.Transaction) error {
	if f.isEndOfEpoch {
		localDistributeRewardsTx, err := f.createDistributeRewardsTx()
		if err != nil {
			return err
		}

		if distributeRewardsTx.Hash != localDistributeRewardsTx.Hash {
			return fmt.Errorf(
				"invalid distribute rewards transaction. Expected '%s', but got '%s' distribute rewards hash",
				localDistributeRewardsTx.Hash,
				distributeRewardsTx.Hash,
			)
		}

		return nil
	}

	return errDistributeRewardsTxNotExpected
}

// verifyDistributeDAOIncentiveTx creates distribute vault rewards transaction
// and compares its hash with the one extracted from the block.
func (f *fsm) verifyDistributeDAOIncentiveTx(distributeDAOIncentiveTx *types.Transaction) error {
	if f.isEndOfEpoch {
		localDistributeDAOIncentiveTx, err := f.createDistributeDAOIncentiveTx()
		if err != nil {
			return err
		}

		if distributeDAOIncentiveTx.Hash != localDistributeDAOIncentiveTx.Hash {
			return fmt.Errorf(
				"invalid distribute DAO incentive rewards transaction. Expected '%s', "+
					"but got '%s' distribute DAO incentive rewards hash",
				localDistributeDAOIncentiveTx.Hash,
				distributeDAOIncentiveTx.Hash,
			)
		}

		return nil
	}

	return errDistributeDAOIncentiveTxNotExpected
}

// verifySyncValidatorsDataTx creates sync validators data transaction
// and compares its hash with the one extracted from the block.
func (f *fsm) verifySyncValidatorsDataTx(
	syncValidatorsDataTx *types.Transaction,
	txIndex int,
) error {
	if f.isStartOfEpoch {
		expectedTxIndex := 0
		if txIndex != expectedTxIndex {
			return InvalidTxIndexErr(expectedTxIndex, txIndex)
		}

		localSyncValidatorsDataTx, err := f.createSyncValidatorsDataTx()
		if err != nil {
			return err
		}

		if syncValidatorsDataTx.Hash != localSyncValidatorsDataTx.Hash {
			return fmt.Errorf(
				"invalid sync validators data transaction. Expected '%s', but got '%s' sync validators data hash",
				localSyncValidatorsDataTx.Hash,
				syncValidatorsDataTx.Hash,
			)
		}

		return nil
	}

	return errSyncValidatorsDataTxNotExpected
}

// verifyBridgeCommitmentTx validates bridge commitment transaction
func verifyBridgeCommitmentTx(blockNumber uint64, txHash types.Hash,
	commitment *CommitmentMessageSigned,
	validators validator.ValidatorSet) error {
	signers, err := validators.Accounts().GetFilteredValidators(commitment.AggSignature.Bitmap)
	if err != nil {
		return fmt.Errorf("failed to retrieve signers for state tx (%s): %w", txHash, err)
	}

	if !validators.HasQuorum(blockNumber, signers.GetAddressesAsSet()) {
		return fmt.Errorf("quorum size not reached for state tx (%s)", txHash)
	}

	commitmentHash, err := commitment.Hash()
	if err != nil {
		return err
	}

	signature, err := bls.UnmarshalSignature(commitment.AggSignature.AggregatedSignature)
	if err != nil {
		return fmt.Errorf("error for state tx (%s) while unmarshaling signature: %w", txHash, err)
	}

	verified := signature.VerifyAggregated(
		signers.GetBlsKeys(),
		commitmentHash.Bytes(),
		signer.DomainStateReceiver,
	)
	if !verified {
		return fmt.Errorf("invalid signature for state tx (%s)", txHash)
	}

	return nil
}

func validateHeaderFields(parent *types.Header, header *types.Header, blockTimeDrift uint64) error {
	// header extra data must be higher or equal to ExtraVanity = 32 in order to be compliant with Ethereum blocks
	if len(header.ExtraData) < ExtraVanity {
		return fmt.Errorf(
			"extra-data shorter than %d bytes (%d)",
			ExtraVanity,
			len(header.ExtraData),
		)
	}
	// verify parent hash
	if parent.Hash != header.ParentHash {
		return fmt.Errorf(
			"incorrect header parent hash (parent=%s, header parent=%s)",
			parent.Hash,
			header.ParentHash,
		)
	}
	// verify parent number
	if header.Number != parent.Number+1 {
		return fmt.Errorf("invalid number")
	}
	// verify time is from the future
	if header.Timestamp > (uint64(time.Now().UTC().Unix()) + blockTimeDrift) {
		return fmt.Errorf(
			"block from the future. block timestamp: %s, configured block time drift %d seconds",
			time.Unix(int64(header.Timestamp), 0).Format(time.RFC3339),
			blockTimeDrift,
		)
	}
	// verify header nonce is zero
	if header.Nonce != types.ZeroNonce {
		return fmt.Errorf("invalid nonce")
	}
	// verify that the gasUsed is <= gasLimit
	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gas limit: have %v, max %v", header.GasUsed, header.GasLimit)
	}
	// verify time has passed
	if header.Timestamp < parent.Timestamp {
		return fmt.Errorf("timestamp older than parent")
	}
	// verify mix digest
	if header.MixHash != HydragonMixDigest {
		return fmt.Errorf("mix digest is not correct")
	}
	// difficulty must be > 0
	if header.Difficulty <= 0 {
		return fmt.Errorf("difficulty should be greater than zero")
	}
	// calculated header hash must be correct
	if header.Hash != types.HeaderHash(header) {
		return fmt.Errorf("invalid header hash")
	}

	return nil
}

// createStateTransactionWithData creates a state transaction
// with provided target address and inputData parameter which is ABI encoded byte array.
func createStateTransactionWithData(
	blockNumber uint64,
	target types.Address,
	inputData []byte,
	value *big.Int,
) *types.Transaction {
	tx := &types.Transaction{
		From:     contracts.SystemCaller,
		To:       &target,
		Type:     types.StateTx,
		Input:    inputData,
		Gas:      types.StateTransactionGasLimit,
		Value:    value,
		GasPrice: big.NewInt(0),
	}

	return tx.ComputeHash(blockNumber)
}

func (f *fsm) isRewardWalletFundTxRequired() bool {
	return f.rewardWalletFundAmount.Cmp(big.NewInt(0)) != 0
}

func (f *fsm) isSyncValidatorsDataTxRequired() bool {
	parentIbftExtraData, err := GetIbftExtra(f.parent.ExtraData)
	if err != nil {
		return false
	}

	return !parentIbftExtraData.Validators.IsEmpty()
}

// func isCommitEpochTx(tx *types.Transaction) bool {
// 	return isToValidatorSetContract(tx) &&
// 		isCommitEpochFunc(tx)

// }

// func isToValidatorSetContract(tx *types.Transaction) bool {
// 	return *tx.To == contracts.ValidatorSetContract
// }

// func isCommitEpochFunc(tx *types.Transaction) bool {
// 	var commitEpochFn contractsapi.CommitEpochChildValidatorSetFn

// 	return bytes.Equal(tx.Input[:4], commitEpochFn.Sig())
// }

func InvalidTxIndexErr(expected int, actual int) error {
	return fmt.Errorf("invalid transaction index. Expected %d, but got %d", expected, actual)
}
