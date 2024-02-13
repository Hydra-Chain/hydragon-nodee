package polybft

import (
	"math/big"
	"testing"

	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/abi"
	"github.com/umbracle/ethgo/jsonrpc"
)

func TestStakeManager_PostBlock(t *testing.T) {
	t.Parallel()

	var (
		allAliases        = []string{"A", "B", "C", "D", "E", "F"}
		initialSetAliases = []string{"A", "B", "C", "D", "E"}
		epoch             = uint64(1)
		block             = uint64(10)
		firstValidator    = uint64(0)
		secondValidator   = uint64(1)
		stakeAmount       = new(big.Int).Mul(big.NewInt(1e18), big.NewInt(155050))
		validatorSetAddr  = types.StringToAddress("0x0001")
		vPowerExp         = &BigNumDecimal{Numerator: big.NewInt(5000), Denominator: big.NewInt(10000)}
	)

	systemStateMockVar := new(systemStateMock)
	systemStateMockVar.On("GetVotingPowerExponent").Return(vPowerExp, nil).Maybe()

	blockchainMockVar := new(blockchainMock)
	blockchainMockVar.On("GetStateProviderForBlock", mock.Anything).Return(new(stateProviderMock), nil).Maybe()
	blockchainMockVar.On("GetSystemState", mock.Anything, mock.Anything).Return(systemStateMockVar)
	blockchainMockVar.On("CurrentHeader").Return(&types.Header{Number: block}, nil).Maybe()

	state := newTestState(t)
	t.Run("PostBlock - unstake to zero", func(t *testing.T) {
		t.Parallel()

		customSystemStateMock := new(systemStateMock)
		customSystemStateMock.On("GetVotingPowerExponent").Return(&BigNumDecimal{Numerator: big.NewInt(5000), Denominator: big.NewInt(10000)}, nil).Once()

		bcMock := new(blockchainMock)
		bcMock.On("GetStateProviderForBlock", mock.Anything).Return(new(stateProviderMock), nil).Twice()
		bcMock.On("GetSystemState", mock.Anything, mock.Anything).Return(customSystemStateMock).Twice()
		bcMock.On("CurrentHeader").Return(&types.Header{Number: block - 1}, nil).Twice()

		validators := validator.NewTestValidatorsWithAliases(t, allAliases)

		// insert initial full validator set
		require.NoError(t, state.StakeStore.insertFullValidatorSet(validatorSetState{
			Validators:  newValidatorStakeMap(validators.GetPublicIdentities(initialSetAliases...)),
			BlockNumber: block - 1,
		}))

		stakeManager, err := newStakeManager(
			hclog.NewNullLogger(),
			state,
			wallet.NewEcdsaSigner(validators.GetValidator("A").Key()),
			types.StringToAddress("0x0001"),
			5,
			bcMock,
			nil,
		)
		require.NoError(t, err)

		receipt := &types.Receipt{
			Logs: []*types.Log{
				createTestLogForStakeChangedEvent(
					t,
					validatorSetAddr,
					validators.GetValidator(initialSetAliases[firstValidator]).Address(),
					big.NewInt(0),
				),
			},
		}

		receipt.SetStatus(types.ReceiptSuccess)

		req := &PostBlockRequest{
			FullBlock: &types.FullBlock{Block: &types.Block{Header: &types.Header{Number: block}},
				Receipts: []*types.Receipt{receipt},
			},
			Epoch: epoch,
		}

		require.NoError(t, stakeManager.PostBlock(req))

		fullValidatorSet, err := state.StakeStore.getFullValidatorSet()
		require.NoError(t, err)

		var firstValidatorMeta *validator.ValidatorMetadata
		firstValidatorMeta = nil

		for _, validator := range fullValidatorSet.Validators {
			if validator.Address.String() == validators.GetValidator(initialSetAliases[firstValidator]).Address().String() {
				firstValidatorMeta = validator
			}
		}

		require.NotNil(t, firstValidatorMeta)
		require.Equal(t, bigZero, firstValidatorMeta.VotingPower)
		require.False(t, firstValidatorMeta.IsActive)
	})
	t.Run("PostBlock - add stake to one validator", func(t *testing.T) {
		t.Parallel()

		systemStateMockVar := new(systemStateMock)
		systemStateMockVar.On("GetVotingPowerExponent").Return(vPowerExp, nil).Once()

		bcMock := new(blockchainMock)
		bcMock.On("CurrentHeader").Return(&types.Header{Number: block - 1}, true).Once()
		bcMock.On("GetStateProviderForBlock", mock.Anything).Return(new(stateProviderMock), nil).Times(3)
		bcMock.On("GetSystemState", mock.Anything, mock.Anything).Return(systemStateMockVar).Times(3)

		validators := validator.NewTestValidatorsWithAliases(t, allAliases)

		state := newTestState(t)

		// insert initial full validator set
		require.NoError(t, state.StakeStore.insertFullValidatorSet(validatorSetState{
			Validators:  newValidatorStakeMap(validators.GetPublicIdentities(initialSetAliases...)),
			BlockNumber: block - 1,
		}))

		stakeManager, err := newStakeManager(
			hclog.NewNullLogger(),
			state,
			wallet.NewEcdsaSigner(validators.GetValidator("A").Key()),
			types.StringToAddress("0x0001"),
			5,
			bcMock,
			nil,
		)
		require.NoError(t, err)

		receipt := &types.Receipt{
			Logs: []*types.Log{
				createTestLogForStakeChangedEvent(
					t,
					validatorSetAddr,
					validators.GetValidator(initialSetAliases[secondValidator]).Address(),
					stakeAmount,
				),
			},
		}

		receipt.SetStatus(types.ReceiptSuccess)

		req := &PostBlockRequest{
			FullBlock: &types.FullBlock{Block: &types.Block{Header: &types.Header{Number: block}},
				Receipts: []*types.Receipt{receipt},
			},
			Epoch: epoch,
		}

		require.NoError(t, stakeManager.PostBlock(req))

		fullValidatorSet, err := state.StakeStore.getFullValidatorSet()
		require.NoError(t, err)

		var firstValidator *validator.ValidatorMetadata = nil

		for _, validator := range fullValidatorSet.Validators {
			if validator.Address.String() == validators.GetValidator(initialSetAliases[secondValidator]).Address().String() {
				firstValidator = validator
			}
		}

		require.NotNil(t, firstValidator)
		require.Equal(t, validator.CalculateVPower(stakeAmount, vPowerExp.Numerator, vPowerExp.Denominator), firstValidator.VotingPower)
		require.True(t, firstValidator.IsActive)
	})

	t.Run("PostBlock - add validator and stake", func(t *testing.T) {
		t.Parallel()

		validators := validator.NewTestValidatorsWithAliases(t, allAliases, []uint64{10, 20, 30, 40, 50, 60})

		bcMock := new(blockchainMock)
		bcMock.On("CurrentHeader").Return(&types.Header{Number: block - 1}, true).Twice()
		bcMock.On("GetStateProviderForBlock", mock.Anything).Return(new(stateProviderMock), nil).Twice()
		bcMock.On("GetSystemState", mock.Anything, mock.Anything).Return(systemStateMockVar).Twice()

		state := newTestState(t)
		// insert initial full validator set
		require.NoError(t, state.StakeStore.insertFullValidatorSet(validatorSetState{
			Validators:  newValidatorStakeMap(validators.GetPublicIdentities(initialSetAliases...)),
			BlockNumber: block - 1,
		}))

		stakeManager, err := newStakeManager(
			hclog.NewNullLogger(),
			state,
			wallet.NewEcdsaSigner(validators.GetValidator("A").Key()),
			types.StringToAddress("0x0001"),
			5,
			bcMock,
			nil,
		)
		require.NoError(t, err)

		receipts := make([]*types.Receipt, len(allAliases))
		for i := 0; i < len(allAliases); i++ {
			receipts[i] = &types.Receipt{Logs: []*types.Log{
				createTestLogForStakeChangedEvent(
					t,
					validatorSetAddr,
					validators.GetValidator(allAliases[i]).Address(),
					stakeAmount,
				),
			}}
			receipts[i].SetStatus(types.ReceiptSuccess)
		}

		req := &PostBlockRequest{
			FullBlock: &types.FullBlock{Block: &types.Block{Header: &types.Header{Number: block}},
				Receipts: receipts},
			Epoch: epoch,
		}

		require.NoError(t, stakeManager.PostBlock(req))

		fullValidatorSet, err := state.StakeStore.getFullValidatorSet()
		require.NoError(t, err)
		require.Len(t, fullValidatorSet.Validators, len(allAliases))

		validatorsCount := validators.ToValidatorSet().Len()
		for _, v := range fullValidatorSet.Validators.getSorted(validatorsCount) {
			require.Equal(t, validator.CalculateVPower(stakeAmount, vPowerExp.Numerator, vPowerExp.Denominator), v.VotingPower)
		}
	})

	t.Run("PostBlock - add stake to one validator + missing block", func(t *testing.T) {
		t.Parallel()

		state := newTestState(t)

		receipt := &types.Receipt{}
		header0 := &types.Header{Hash: types.Hash{3, 5}, Number: block - 3}
		header1 := &types.Header{Hash: types.Hash{3, 2}, Number: block - 2}
		header2 := &types.Header{Hash: types.Hash{6, 4}, Number: block - 1}

		systemStateMockVar := new(systemStateMock)
		systemStateMockVar.On("GetVotingPowerExponent").Return(vPowerExp, nil).Once()

		bcMock := new(blockchainMock)
		bcMock.On("CurrentHeader").Return(header0)
		bcMock.On("GetHeaderByNumber", block-2).Return(header1, true).Once()
		bcMock.On("GetHeaderByNumber", block-1).Return(header2, true).Once()
		bcMock.On("GetReceiptsByHash", header1.Hash).Return([]*types.Receipt{receipt}, error(nil)).Once()
		bcMock.On("GetReceiptsByHash", header2.Hash).Return([]*types.Receipt{}, error(nil)).Once()
		bcMock.On("GetStateProviderForBlock", mock.Anything).Return(new(stateProviderMock), nil).Once()
		bcMock.On("GetSystemState", mock.Anything, mock.Anything).Return(systemStateMockVar).Once()

		validators := validator.NewTestValidatorsWithAliases(t, allAliases)

		// insert initial full validator set
		require.NoError(t, state.StakeStore.insertFullValidatorSet(validatorSetState{
			Validators:  newValidatorStakeMap(validators.GetPublicIdentities(initialSetAliases...)),
			BlockNumber: block - 3,
		}))

		stakeManager, err := newStakeManager(
			hclog.NewNullLogger(),
			state,
			wallet.NewEcdsaSigner(validators.GetValidator("A").Key()),
			types.StringToAddress("0x0001"),
			5,
			bcMock,
			nil,
		)
		require.NoError(t, err)

		customStakeAmount := big.NewInt(0).Add(stakeAmount, big.NewInt(250))
		receipt.Logs = []*types.Log{
			createTestLogForStakeChangedEvent(
				t,
				validatorSetAddr,
				validators.GetValidator(initialSetAliases[secondValidator]).Address(),
				customStakeAmount,
			),
		}
		receipt.SetStatus(types.ReceiptSuccess)

		req := &PostBlockRequest{
			FullBlock: &types.FullBlock{Block: &types.Block{Header: &types.Header{Number: block}},
				Receipts: []*types.Receipt{receipt},
			},
			Epoch: epoch,
		}

		require.NoError(t, stakeManager.PostBlock(req))

		fullValidatorSet, err := state.StakeStore.getFullValidatorSet()
		require.NoError(t, err)

		var updatedValidator *validator.ValidatorMetadata
		updatedValidator = nil

		for _, validator := range fullValidatorSet.Validators {
			if validator.Address.String() == validators.GetValidator(initialSetAliases[secondValidator]).Address().String() {
				updatedValidator = validator
			}
		}

		require.NotNil(t, updatedValidator)
		require.Equal(
			t,
			validator.CalculateVPower(customStakeAmount, vPowerExp.Numerator, vPowerExp.Denominator),
			updatedValidator.VotingPower,
		)
		require.True(t, updatedValidator.IsActive)

		bcMock.AssertExpectations(t)
	})
}

func TestStakeManager_UpdateValidatorSet(t *testing.T) {
	var (
		aliases = []string{"A", "B", "C", "D", "E"}
		stakes  = []uint64{10, 10, 10, 10, 10}
		epoch   = uint64(1)
	)

	validators := validator.NewTestValidatorsWithAliases(t, aliases, stakes)
	state := newTestState(t)

	bcMock := new(blockchainMock)
	bcMock.On("CurrentHeader").Return(&types.Header{Number: 0}, true).Once()

	require.NoError(t, state.StakeStore.insertFullValidatorSet(validatorSetState{
		Validators: newValidatorStakeMap(validators.ToValidatorSet().Accounts()),
	}))

	stakeManager, err := newStakeManager(
		hclog.NewNullLogger(),
		state,
		wallet.NewEcdsaSigner(validators.GetValidator("A").Key()),
		types.StringToAddress("0x0001"),
		10,
		bcMock,
		nil,
	)
	require.NoError(t, err)

	t.Run("UpdateValidatorSet - only update", func(t *testing.T) {
		fullValidatorSet := validators.GetPublicIdentities().Copy()
		validatorToUpdate := fullValidatorSet[0]
		validatorToUpdate.VotingPower = big.NewInt(11)

		require.NoError(t, state.StakeStore.insertFullValidatorSet(validatorSetState{
			Validators: newValidatorStakeMap(fullValidatorSet),
		}))

		updateDelta, err := stakeManager.UpdateValidatorSet(epoch, validators.GetPublicIdentities())
		require.NoError(t, err)
		require.Len(t, updateDelta.Added, 0)
		require.Len(t, updateDelta.Updated, 1)
		require.Len(t, updateDelta.Removed, 0)
		require.Equal(t, updateDelta.Updated[0].Address, validatorToUpdate.Address)
		require.Equal(t, updateDelta.Updated[0].VotingPower.Uint64(), validatorToUpdate.VotingPower.Uint64())
	})

	t.Run("UpdateValidatorSet - one unstake", func(t *testing.T) {
		fullValidatorSet := validators.GetPublicIdentities(aliases[1:]...)

		require.NoError(t, state.StakeStore.insertFullValidatorSet(validatorSetState{
			Validators: newValidatorStakeMap(fullValidatorSet),
		}))

		updateDelta, err := stakeManager.UpdateValidatorSet(epoch+1, validators.GetPublicIdentities())
		require.NoError(t, err)
		require.Len(t, updateDelta.Added, 0)
		require.Len(t, updateDelta.Updated, 0)
		require.Len(t, updateDelta.Removed, 1)
	})

	t.Run("UpdateValidatorSet - one new validator", func(t *testing.T) {
		addedValidator := validators.GetValidator("A")

		require.NoError(t, state.StakeStore.insertFullValidatorSet(validatorSetState{
			Validators: newValidatorStakeMap(validators.GetPublicIdentities()),
		}))

		updateDelta, err := stakeManager.UpdateValidatorSet(epoch+2,
			validators.GetPublicIdentities(aliases[1:]...))
		require.NoError(t, err)
		require.Len(t, updateDelta.Added, 1)
		require.Len(t, updateDelta.Updated, 0)
		require.Len(t, updateDelta.Removed, 0)
		require.Equal(t, addedValidator.Address(), updateDelta.Added[0].Address)
		require.Equal(t, addedValidator.VotingPower, updateDelta.Added[0].VotingPower.Uint64())
	})
	t.Run("UpdateValidatorSet - remove some stake", func(t *testing.T) {
		fullValidatorSet := validators.GetPublicIdentities().Copy()
		validatorToUpdate := fullValidatorSet[2]
		validatorToUpdate.VotingPower = big.NewInt(5)

		require.NoError(t, state.StakeStore.insertFullValidatorSet(validatorSetState{
			Validators: newValidatorStakeMap(fullValidatorSet),
		}))

		updateDelta, err := stakeManager.UpdateValidatorSet(epoch+3, validators.GetPublicIdentities())
		require.NoError(t, err)
		require.Len(t, updateDelta.Added, 0)
		require.Len(t, updateDelta.Updated, 1)
		require.Len(t, updateDelta.Removed, 0)
		require.Equal(t, updateDelta.Updated[0].Address, validatorToUpdate.Address)
		require.Equal(t, updateDelta.Updated[0].VotingPower.Uint64(), validatorToUpdate.VotingPower.Uint64())
	})
	t.Run("UpdateValidatorSet - remove entire stake", func(t *testing.T) {
		fullValidatorSet := validators.GetPublicIdentities().Copy()
		validatorToUpdate := fullValidatorSet[3]
		validatorToUpdate.VotingPower = bigZero

		require.NoError(t, state.StakeStore.insertFullValidatorSet(validatorSetState{
			Validators: newValidatorStakeMap(fullValidatorSet),
		}))

		updateDelta, err := stakeManager.UpdateValidatorSet(epoch+4, validators.GetPublicIdentities())
		require.NoError(t, err)
		require.Len(t, updateDelta.Added, 0)
		require.Len(t, updateDelta.Updated, 0)
		require.Len(t, updateDelta.Removed, 1)
	})
	t.Run("UpdateValidatorSet - voting power negative", func(t *testing.T) {
		fullValidatorSet := validators.GetPublicIdentities().Copy()
		validatorsToUpdate := fullValidatorSet[4]
		validatorsToUpdate.VotingPower = bigZero

		require.NoError(t, state.StakeStore.insertFullValidatorSet(validatorSetState{
			Validators: newValidatorStakeMap(fullValidatorSet),
		}))

		updateDelta, err := stakeManager.UpdateValidatorSet(epoch+5, validators.GetPublicIdentities())
		require.NoError(t, err)
		require.Len(t, updateDelta.Added, 0)
		require.Len(t, updateDelta.Updated, 0)
		require.Len(t, updateDelta.Removed, 1)
	})

	t.Run("UpdateValidatorSet - max validator set size reached", func(t *testing.T) {
		// because we now have 5 validators, and the new validator has more stake
		stakeManager.maxValidatorSetSize = 4

		fullValidatorSet := validators.GetPublicIdentities().Copy()
		validatorToAdd := fullValidatorSet[0]
		validatorToAdd.VotingPower = big.NewInt(11)

		require.NoError(t, state.StakeStore.insertFullValidatorSet(validatorSetState{
			Validators: newValidatorStakeMap(fullValidatorSet),
		}))

		updateDelta, err := stakeManager.UpdateValidatorSet(epoch+6,
			validators.GetPublicIdentities(aliases[1:]...))

		require.NoError(t, err)
		require.Len(t, updateDelta.Added, 1)
		require.Len(t, updateDelta.Updated, 0)
		require.Len(t, updateDelta.Removed, 1)
		require.Equal(t, validatorToAdd.Address, updateDelta.Added[0].Address)
		require.Equal(t, validatorToAdd.VotingPower.Uint64(), updateDelta.Added[0].VotingPower.Uint64())
	})
}

func TestStakeCounter_ShouldBeDeterministic(t *testing.T) {
	t.Parallel()

	const timesToExecute = 100

	stakes := [][]uint64{
		{103, 102, 101, 51, 50, 30, 10},
		{100, 100, 100, 50, 50, 30, 10},
		{103, 102, 101, 51, 50, 30, 10},
		{100, 100, 100, 50, 50, 30, 10},
	}
	maxValidatorSetSizes := []int{1000, 1000, 5, 6}

	for ind, stake := range stakes {
		maxValidatorSetSize := maxValidatorSetSizes[ind]

		aliases := []string{"A", "B", "C", "D", "E", "F", "G"}
		validators := validator.NewTestValidatorsWithAliases(t, aliases, stake)

		test := func() []*validator.ValidatorMetadata {
			stakeCounter := newValidatorStakeMap(validators.GetPublicIdentities("A", "B", "C", "D", "E"))

			return stakeCounter.getSorted(maxValidatorSetSize)
		}

		initialSlice := test()

		// stake counter and stake map should always be deterministic
		for i := 0; i < timesToExecute; i++ {
			currentSlice := test()

			require.Len(t, currentSlice, len(initialSlice))

			for i, si := range currentSlice {
				initialSi := initialSlice[i]
				require.Equal(t, si.Address, initialSi.Address)
				require.Equal(t, si.VotingPower.Uint64(), initialSi.VotingPower.Uint64())
			}
		}
	}
}

func TestStakeManager_UpdateOnInit(t *testing.T) {
	t.Parallel()

	var (
		allAliases       = []string{"A", "B", "C", "D", "E", "F"}
		validatorSetAddr = types.StringToAddress("0xf001")
		epochID          = uint64(120)
		stakeAmount      = new(big.Int).Mul(big.NewInt(1e18), big.NewInt(155050))
		vPowerExp        = &BigNumDecimal{Numerator: big.NewInt(5000), Denominator: big.NewInt(10000)}
	)

	success := types.ReceiptSuccess
	contractProvider := &stateProvider{}
	header1Hash := types.StringToHash("0x99aa")
	header2Hash := types.StringToHash("0xffee")
	header3Hash := types.StringToHash("0xeeff")
	header4Hash := types.StringToHash("0xaaff")
	currentHeader := &types.Header{Number: 4}
	validators := validator.NewTestValidatorsWithAliases(t, allAliases)
	accountSet := validators.GetPublicIdentities(allAliases...)
	addresses := accountSet.GetAddresses()
	state := newTestState(t)

	sysStateMock := &systemStateMock{}
	sysStateMock.On("GetEpoch").Return(epochID, nil).Once()
	sysStateMock.On("GetVotingPowerExponent").Return(vPowerExp, nil).Once()

	polyBackendMock := new(polybftBackendMock)
	polyBackendMock.On("GetValidators", uint64(0), []*types.Header(nil)).Return(accountSet, nil).Once()

	bcMock := new(blockchainMock)
	bcMock.On("GetStateProviderForBlock", currentHeader).Return(contractProvider, nil).Twice()
	bcMock.On("GetSystemState", contractProvider).Return(sysStateMock, nil).Twice()
	bcMock.On("CurrentHeader", mock.Anything).Return(currentHeader, true).Once()
	bcMock.On("GetHeaderByNumber", uint64(1)).Return(&types.Header{Number: 1, Hash: header1Hash}, true).Once()
	bcMock.On("GetHeaderByNumber", uint64(2)).Return(&types.Header{Number: 2, Hash: header2Hash}, true).Once()
	bcMock.On("GetHeaderByNumber", uint64(3)).Return(&types.Header{Number: 3, Hash: header3Hash}, true).Once()
	bcMock.On("GetHeaderByNumber", uint64(4)).Return(&types.Header{Number: 4, Hash: header4Hash}, true).Once()
	bcMock.On("GetReceiptsByHash", header1Hash).Return([]*types.Receipt(nil), nil).Once()

	stakeAmountTwo := new(big.Int).Mul(stakeAmount, big.NewInt(2))

	bcMock.On("GetReceiptsByHash", header2Hash).Return([]*types.Receipt{
		{
			Status: &success,
			Logs: []*types.Log{
				createTestLogForStakeChangedEvent(
					t,
					validatorSetAddr,
					addresses[len(addresses)-2],
					stakeAmountTwo,
				),
			},
		},
	}, nil).Once()

	stakeAmountThree := new(big.Int).Mul(stakeAmount, big.NewInt(3))

	bcMock.On("GetReceiptsByHash", header3Hash).Return([]*types.Receipt{
		{
			Status: &success,
			Logs: []*types.Log{
				createTestLogForStakeChangedEvent(
					t,
					validatorSetAddr,
					addresses[len(addresses)-1],
					stakeAmountThree,
				),
			},
		},
	}, nil).Once()
	bcMock.On("GetReceiptsByHash", header4Hash).Return([]*types.Receipt{{}}, nil).Once()

	_, err := newStakeManager(
		hclog.NewNullLogger(),
		state,
		wallet.NewEcdsaSigner(validators.GetValidator("A").Key()),
		validatorSetAddr,
		5,
		bcMock,
		polyBackendMock,
	)
	require.NoError(t, err)

	bcMock.AssertExpectations(t)
	sysStateMock.AssertExpectations(t)

	fullValidatorSet, err := state.StakeStore.getFullValidatorSet()
	require.NoError(t, err)

	require.Equal(t, uint64(4), fullValidatorSet.BlockNumber)
	require.Equal(t, uint64(4), fullValidatorSet.UpdatedAtBlockNumber)
	require.Equal(t, epochID, fullValidatorSet.EpochID)

	for _, x := range fullValidatorSet.Validators {
		if x.Address == addresses[len(addresses)-1] {
			require.Equal(t, validator.CalculateVPower(stakeAmountThree, vPowerExp.Numerator, vPowerExp.Denominator), x.VotingPower)
		} else if x.Address == addresses[len(addresses)-2] {
			require.Equal(t, validator.CalculateVPower(stakeAmountTwo, vPowerExp.Numerator, vPowerExp.Denominator), x.VotingPower)
		} else {
			require.Equal(t, big.NewInt(15000), x.VotingPower)
		}
	}
}

func createTestLogForStakeChangedEvent(t *testing.T, validatorSet, validator types.Address, stake *big.Int) *types.Log {
	t.Helper()

	var stakeChangedEvent contractsapi.StakeChangedEvent

	topics := make([]types.Hash, 2)
	topics[0] = types.Hash(stakeChangedEvent.Sig())
	topics[1] = types.BytesToHash(validator.Bytes())
	encodedData, err := abi.MustNewType("uint256").Encode(stake)
	require.NoError(t, err)

	return &types.Log{
		Address: validatorSet,
		Topics:  topics,
		Data:    encodedData,
	}
}

var _ txrelayer.TxRelayer = (*dummyStakeTxRelayer)(nil)

type dummyStakeTxRelayer struct {
	mock.Mock
	callback func() *validator.ValidatorMetadata
	t        *testing.T
}

func newDummyStakeTxRelayer(t *testing.T, callback func() *validator.ValidatorMetadata) *dummyStakeTxRelayer {
	t.Helper()

	return &dummyStakeTxRelayer{
		t:        t,
		callback: callback,
	}
}

func (d *dummyStakeTxRelayer) Call(from ethgo.Address, to ethgo.Address, input []byte) (string, error) {
	args := d.Called(from, to, input)

	if d.callback != nil {
		validatorMetaData := d.callback()
		encoded, err := validatorTypeABI.Encode(map[string]interface{}{
			"blsKey":        validatorMetaData.BlsKey.ToBigInt(),
			"stake":         validatorMetaData.VotingPower,
			"isWhitelisted": true,
			"isActive":      true,
		})

		require.NoError(d.t, err)

		return hex.EncodeToHex(encoded), nil
	}

	return args.String(0), args.Error(1)
}

func (d *dummyStakeTxRelayer) SendTransaction(transaction *ethgo.Transaction, key ethgo.Key) (*ethgo.Receipt, error) {
	args := d.Called(transaction, key)

	return args.Get(0).(*ethgo.Receipt), args.Error(1) //nolint:forcetypeassert
}

// SendTransactionLocal sends non-signed transaction (this is only for testing purposes)
func (d *dummyStakeTxRelayer) SendTransactionLocal(txn *ethgo.Transaction) (*ethgo.Receipt, error) {
	args := d.Called(txn)

	return args.Get(0).(*ethgo.Receipt), args.Error(1) //nolint:forcetypeassert
}

func (d *dummyStakeTxRelayer) Client() *jsonrpc.Client {
	return nil
}
