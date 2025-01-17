package genesis

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/0xPolygon/polygon-edge/chain"
	"github.com/0xPolygon/polygon-edge/command"
	"github.com/0xPolygon/polygon-edge/command/helper"
	"github.com/0xPolygon/polygon-edge/consensus/ibft"
	"github.com/0xPolygon/polygon-edge/consensus/ibft/fork"
	"github.com/0xPolygon/polygon-edge/consensus/ibft/signer"
	"github.com/0xPolygon/polygon-edge/consensus/polybft"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/contracts/staking"
	stakingHelper "github.com/0xPolygon/polygon-edge/helper/staking"
	"github.com/0xPolygon/polygon-edge/secrets"
	"github.com/0xPolygon/polygon-edge/server"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/0xPolygon/polygon-edge/validators"
)

const (
	dirFlag           = "dir"
	nameFlag          = "name"
	premineFlag       = "premine"
	chainIDFlag       = "chain-id"
	epochSizeFlag     = "epoch-size"
	epochRewardFlag   = "epoch-reward"
	blockGasLimitFlag = "block-gas-limit"
	// Hydra modification: we use the 0x0 address for burning, thus, we don't need the this flag
	// burnContractFlag             = "burn-contract"
	genesisBaseFeeConfigFlag = "base-fee-config"
	posFlag                  = "pos"
	// Hydra modification: we don't use a separate native erc20 token, thus, we don't need this flag
	// nativeTokenConfigFlag        = "native-token-config"
	blockTrackerPollIntervalFlag = "block-tracker-poll-interval"
	proxyContractsAdminFlag      = "proxy-contracts-admin"
	governanceFlag               = "governance"
)

// Legacy flags that need to be preserved for running clients
const (
	chainIDFlagLEGACY = "chainid"
)

var (
	params = &genesisParams{}
)

var (
	errValidatorsNotSpecified   = errors.New("validator information not specified")
	errUnsupportedConsensus     = errors.New("specified consensusRaw not supported")
	errInvalidEpochSize         = errors.New("epoch size must be greater than 1")
	errReserveAccMustBePremined = errors.New(
		"it is mandatory to premine reserve account (0x0 address)",
	)
	errBlockTrackerPollInterval = errors.New("block tracker poll interval must be greater than 0")
	errBaseFeeChangeDenomZero   = errors.New("base fee change denominator must be greater than 0")
	errBaseFeeEMZero            = errors.New(
		"base fee elasticity multiplier must be greater than 0",
	)
	errBaseFeeZero = errors.New("base fee must be greater than 0")
)

type genesisParams struct {
	genesisPath  string
	name         string
	consensusRaw string
	premine      []string
	bootnodes    []string

	chainID   uint64
	epochSize uint64

	blockGasLimit uint64

	burnContract        string
	baseFeeConfig       string
	parsedBaseFeeConfig *baseFeeInfo

	// PoS
	isPos                bool
	minNumValidators     uint64
	maxNumValidators     uint64
	validatorsPath       string
	validatorsPrefixPath string
	validators           []string

	// IBFT
	rawIBFTValidatorType string
	ibftValidatorType    validators.ValidatorType
	ibftValidators       validators.Validators

	extraData []byte
	consensus server.ConsensusType

	consensusEngineConfig map[string]interface{}

	genesisConfig *chain.Chain

	// PolyBFT
	sprintSize     uint64
	blockTime      time.Duration
	epochReward    uint64
	blockTimeDrift uint64

	initialStateRoot string

	// access lists
	contractDeployerAllowListAdmin   []string
	contractDeployerAllowListEnabled []string
	contractDeployerBlockListAdmin   []string
	contractDeployerBlockListEnabled []string
	transactionsAllowListAdmin       []string
	transactionsAllowListEnabled     []string
	transactionsBlockListAdmin       []string
	transactionsBlockListEnabled     []string
	bridgeAllowListAdmin             []string
	bridgeAllowListEnabled           []string
	bridgeBlockListAdmin             []string
	bridgeBlockListEnabled           []string

	nativeTokenConfigRaw string
	nativeTokenConfig    *polybft.TokenConfig

	premineInfos []*helper.PremineInfo

	blockTrackerPollInterval time.Duration

	proxyContractsAdmin string
	governance          string

	secretsConfigPath string
	secretsConfig     *secrets.SecretsManagerConfig
}

func (p *genesisParams) validateFlags() error {
	// Check if the consensusRaw is supported
	if !server.ConsensusSupported(p.consensusRaw) {
		return errUnsupportedConsensus
	}

	if err := p.validateGenesisBaseFeeConfig(); err != nil {
		return err
	}

	// Check if validator information is set at all
	if p.isIBFTConsensus() &&
		!p.areValidatorsSetManually() &&
		!p.areValidatorsSetByPrefix() {
		return errValidatorsNotSpecified
	}

	if err := p.parsePremineInfo(); err != nil {
		return err
	}

	if p.isPolyBFTConsensus() { //nolint:wsl
		// Hydra modification: we don't use a separate native erc20 token neither a burn contract,
		// thus, we don't need the following validations
		// if err := p.extractNativeTokenMetadata(); err != nil {
		// 	return err
		// }

		// if err := p.validateBurnContract(); err != nil {
		// 	return err
		// }

		// if err := p.validatePremineInfo(); err != nil {
		// 	return err
		// }

		if err := p.validateProxyContractsAdmin(); err != nil {
			return err
		}

		if err := p.validateGovernanceAddress(); err != nil {
			return err
		}
	}

	// Check if the genesis file already exists
	if generateError := verifyGenesisExistence(p.genesisPath); generateError != nil {
		return errors.New(generateError.GetMessage())
	}

	// Check that the epoch size is correct
	if p.epochSize < 2 && (p.isIBFTConsensus() || p.isPolyBFTConsensus()) {
		// Epoch size must be greater than 1, so new transactions have a chance to be added to a block.
		// Otherwise, every block would be an endblock (meaning it will not have any transactions).
		// Check is placed here to avoid additional parsing if epochSize < 2
		return errInvalidEpochSize
	}

	// Validate validatorsPath only if validators information were not provided via CLI flag
	if len(p.validators) == 0 {
		if _, err := os.Stat(p.validatorsPath); err != nil {
			return fmt.Errorf(
				"invalid validators path ('%s') provided. Error: %w",
				p.validatorsPath,
				err,
			)
		}
	}

	// Validate min and max validators number
	return command.ValidateMinMaxValidatorsNumber(p.minNumValidators, p.maxNumValidators)
}

func (p *genesisParams) isIBFTConsensus() bool {
	return server.ConsensusType(p.consensusRaw) == server.IBFTConsensus
}

func (p *genesisParams) isPolyBFTConsensus() bool {
	return server.ConsensusType(p.consensusRaw) == server.PolyBFTConsensus
}

func (p *genesisParams) areValidatorsSetManually() bool {
	return len(p.validators) != 0
}

func (p *genesisParams) areValidatorsSetByPrefix() bool {
	return p.validatorsPrefixPath != ""
}

func (p *genesisParams) getRequiredFlags() []string {
	if p.isIBFTConsensus() {
		return []string{
			command.BootnodeFlag,
			governanceFlag,
		}
	}

	return []string{}
}

func (p *genesisParams) initRawParams() error {
	p.consensus = server.ConsensusType(p.consensusRaw)

	if p.consensus == server.PolyBFTConsensus {
		return nil
	}

	if err := p.initIBFTValidatorType(); err != nil {
		return err
	}

	if err := p.initValidatorSet(); err != nil {
		return err
	}

	p.initIBFTExtraData()
	p.initConsensusEngineConfig()

	return nil
}

// setValidatorSetFromCli sets validator set from cli command
func (p *genesisParams) setValidatorSetFromCli() error {
	if len(p.validators) == 0 {
		return nil
	}

	newValidators, err := validators.ParseValidators(p.ibftValidatorType, p.validators)
	if err != nil {
		return err
	}

	if err = p.ibftValidators.Merge(newValidators); err != nil {
		return err
	}

	return nil
}

// setValidatorSetFromPrefixPath sets validator set from prefix path
func (p *genesisParams) setValidatorSetFromPrefixPath() error {
	if !p.areValidatorsSetByPrefix() {
		return nil
	}

	validators, err := command.GetValidatorsFromPrefixPath(
		p.validatorsPath,
		p.validatorsPrefixPath,
		p.ibftValidatorType,
	)

	if err != nil {
		return fmt.Errorf("failed to read from prefix: %w", err)
	}

	if err := p.ibftValidators.Merge(validators); err != nil {
		return err
	}

	return nil
}

func (p *genesisParams) initIBFTValidatorType() error {
	var err error
	if p.ibftValidatorType, err = validators.ParseValidatorType(p.rawIBFTValidatorType); err != nil {
		return err
	}

	return nil
}

func (p *genesisParams) initValidatorSet() error {
	p.ibftValidators = validators.NewValidatorSetFromType(p.ibftValidatorType)

	// Set the initial validators
	// Priority goes to cli command over prefix path
	if err := p.setValidatorSetFromPrefixPath(); err != nil {
		return err
	}

	if err := p.setValidatorSetFromCli(); err != nil {
		return err
	}

	// Validate if validator number exceeds max number
	if ok := p.isValidatorNumberValid(); !ok {
		return command.ErrValidatorNumberExceedsMax
	}

	return nil
}

func (p *genesisParams) isValidatorNumberValid() bool {
	return p.ibftValidators == nil || uint64(p.ibftValidators.Len()) <= p.maxNumValidators //nolint:gosec
}

func (p *genesisParams) initIBFTExtraData() {
	if p.consensus != server.IBFTConsensus {
		return
	}

	var committedSeal signer.Seals

	switch p.ibftValidatorType {
	case validators.ECDSAValidatorType:
		committedSeal = new(signer.SerializedSeal)
	case validators.BLSValidatorType:
		committedSeal = new(signer.AggregatedSeal)
	}

	ibftExtra := &signer.IstanbulExtra{
		Validators:     p.ibftValidators,
		ProposerSeal:   []byte{},
		CommittedSeals: committedSeal,
	}

	p.extraData = make([]byte, signer.IstanbulExtraVanity)
	p.extraData = ibftExtra.MarshalRLPTo(p.extraData)
}

func (p *genesisParams) initConsensusEngineConfig() {
	if p.consensus != server.IBFTConsensus {
		p.consensusEngineConfig = map[string]interface{}{
			p.consensusRaw: map[string]interface{}{},
		}

		return
	}

	if p.isPos {
		p.initIBFTEngineMap(fork.PoS)

		return
	}

	p.initIBFTEngineMap(fork.PoA)
}

func (p *genesisParams) initIBFTEngineMap(ibftType fork.IBFTType) {
	p.consensusEngineConfig = map[string]interface{}{
		string(server.IBFTConsensus): map[string]interface{}{
			fork.KeyType:          ibftType,
			fork.KeyValidatorType: p.ibftValidatorType,
			fork.KeyBlockTime:     p.blockTime,
			ibft.KeyEpochSize:     p.epochSize,
		},
	}
}

func (p *genesisParams) generateGenesis() error {
	if err := p.initGenesisConfig(); err != nil {
		return err
	}

	if err := helper.WriteGenesisConfigToDisk(
		p.genesisConfig,
		p.genesisPath,
	); err != nil {
		return err
	}

	return nil
}

func (p *genesisParams) initGenesisConfig() error {
	// Disable london hardfork if burn contract address is not provided
	enabledForks := chain.AllForksEnabled
	// Hydra modification: london hardfork is enabled no matter the burn contract state
	// if !p.isBurnContractEnabled() {
	// 	enabledForks.RemoveFork(chain.London)
	// }

	chainConfig := &chain.Chain{
		Name: p.name,
		Genesis: &chain.Genesis{
			GasLimit:   p.blockGasLimit,
			Difficulty: 1,
			Alloc:      map[types.Address]*chain.GenesisAccount{},
			ExtraData:  p.extraData,
			GasUsed:    command.DefaultGenesisGasUsed,
		},
		Params: &chain.Params{
			ChainID:        int64(p.chainID), //nolint:gosec
			Forks:          enabledForks,
			Engine:         p.consensusEngineConfig,
			BlockGasTarget: 100000000,
		},
		Bootnodes: p.bootnodes,
	}

	// burn contract can be set only for non mintable native token
	// Hydra modification: set eip1559 fee flags
	chainConfig.Genesis.BaseFee = p.parsedBaseFeeConfig.baseFee
	chainConfig.Genesis.BaseFeeEM = p.parsedBaseFeeConfig.baseFeeEM
	chainConfig.Genesis.BaseFeeChangeDenom = p.parsedBaseFeeConfig.baseFeeChangeDenom

	// Predeploy staking smart contract if needed
	if p.shouldPredeployStakingSC() {
		stakingAccount, err := p.predeployStakingSC()
		if err != nil {
			return err
		}

		chainConfig.Genesis.Alloc[staking.AddrStakingContract] = stakingAccount
	}

	for _, premineInfo := range p.premineInfos {
		chainConfig.Genesis.Alloc[premineInfo.Address] = &chain.GenesisAccount{
			Balance: premineInfo.Amount,
		}
	}

	p.genesisConfig = chainConfig

	return nil
}

func (p *genesisParams) shouldPredeployStakingSC() bool {
	// If the consensus selected is IBFT / Dev and the mechanism is Proof of Stake,
	// deploy the Staking SC
	return p.isPos && (p.consensus == server.IBFTConsensus || p.consensus == server.DevConsensus)
}

func (p *genesisParams) predeployStakingSC() (*chain.GenesisAccount, error) {
	stakingAccount, predeployErr := stakingHelper.PredeployStakingSC(
		p.ibftValidators,
		stakingHelper.PredeployParams{
			MinValidatorCount: p.minNumValidators,
			MaxValidatorCount: p.maxNumValidators,
		})
	if predeployErr != nil {
		return nil, predeployErr
	}

	return stakingAccount, nil
}

// parsePremineInfo parses premine flag
func (p *genesisParams) parsePremineInfo() error {
	p.premineInfos = make([]*helper.PremineInfo, 0, len(p.premine))

	for _, premine := range p.premine {
		premineInfo, err := helper.ParsePremineInfo(premine)
		if err != nil {
			return fmt.Errorf("invalid premine balance amount provided: %w", err)
		}

		p.premineInfos = append(p.premineInfos, premineInfo)
	}

	return nil
}

// validatePremineInfo validates whether reserve account (0x0 address) is premined
func (p *genesisParams) validatePremineInfo() error {
	for _, premineInfo := range p.premineInfos {
		if premineInfo.Address == types.ZeroAddress {
			// we have premine of zero address, just return
			return nil
		}
	}

	return errReserveAccMustBePremined
}

// validateBlockTrackerPollInterval validates block tracker block interval
// which can not be 0
func (p *genesisParams) validateBlockTrackerPollInterval() error {
	if p.blockTrackerPollInterval == 0 {
		return helper.ErrBlockTrackerPollInterval
	}

	return nil
}

// Hydra modification: we use the 0x0 address for burning, thus, we don't need this validation function
// validateBurnContract validates burn contract. If native token is mintable,
// burn contract flag must not be set. If native token is non mintable only one burn contract
// can be set and the specified address will be used to predeploy default EIP1559 burn contract.
// func (p *genesisParams) validateBurnContract() error {
// 	if p.isBurnContractEnabled() {
// 		burnContractInfo, err := parseBurnContractInfo(p.burnContract)
// 		if err != nil {
// 			return fmt.Errorf("invalid burn contract info provided: %w", err)
// 		}

// 		if p.nativeTokenConfig.IsMintable {
// 			if burnContractInfo.Address != types.ZeroAddress {
// 				return errors.New(
// 					"only zero address is allowed as burn destination for mintable native token",
// 				)
// 			}
// 		} else {
// 			if burnContractInfo.Address == types.ZeroAddress {
// 				return errors.New("it is not allowed to deploy burn contract to 0x0 address")
// 			}
// 		}
// 	}

// 	return nil
// }

func (p *genesisParams) validateGenesisBaseFeeConfig() error {
	if p.baseFeeConfig == "" {
		return errors.New("invalid input(empty string) for genesis base fee config flag")
	}

	baseFeeInfo, err := parseBaseFeeConfig(p.baseFeeConfig)
	if err != nil {
		return fmt.Errorf(
			"failed to parse base fee config: %w, provided value %s",
			err,
			p.baseFeeConfig,
		)
	}

	p.parsedBaseFeeConfig = baseFeeInfo

	if baseFeeInfo.baseFee == 0 {
		return errBaseFeeZero
	}

	if baseFeeInfo.baseFeeEM == 0 {
		return errBaseFeeEMZero
	}

	if baseFeeInfo.baseFeeChangeDenom == 0 {
		return errBaseFeeChangeDenomZero
	}

	return nil
}

func (p *genesisParams) validateProxyContractsAdmin() error {
	if err := command.ValidateAddress("proxy contracts admin", p.proxyContractsAdmin); err != nil {
		return err
	}

	proxyContractsAdminAddr := types.StringToAddress(p.proxyContractsAdmin)
	if proxyContractsAdminAddr == types.ZeroAddress {
		return errors.New("proxy contracts admin address must not be zero address")
	}

	if proxyContractsAdminAddr == contracts.SystemCaller {
		return errors.New("proxy contracts admin address must not be system caller address")
	}

	return nil
}

// Hydra modification: we use the 0x0 address for burning and not using a separate native erc20 token,
// thus, we don't need the following two functions
// isBurnContractEnabled returns true in case burn contract info is provided
// func (p *genesisParams) isBurnContractEnabled() bool {
// 	return p.burnContract != ""
// }

// extractNativeTokenMetadata parses provided native token metadata (such as name, symbol and decimals count)
// func (p *genesisParams) extractNativeTokenMetadata() error {
// 	tokenConfig, err := polybft.ParseRawTokenConfig(p.nativeTokenConfigRaw)
// 	if err != nil {
// 		return err
// 	}

// 	p.nativeTokenConfig = tokenConfig

// 	return nil
// }

func (p *genesisParams) getResult() command.CommandResult {
	return &GenesisResult{
		Message: fmt.Sprintf("\nGenesis written to %s\n", p.genesisPath),
	}
}

func (p *genesisParams) validateGovernanceAddress() error {
	if err := command.ValidateAddress("governance", p.governance); err != nil {
		return err
	}

	governanceAddr := types.StringToAddress(p.governance)
	if governanceAddr == types.ZeroAddress {
		return errors.New("governance address must not be zero address")
	}

	if governanceAddr == contracts.SystemCaller {
		return errors.New("governance address must not be system caller address")
	}

	if p.proxyContractsAdmin == p.governance {
		return errors.New("governance address must be different than the proxy contracts admin")
	}

	return nil
}
