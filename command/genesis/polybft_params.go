package genesis

import (
	"errors"
	"fmt"
	"math/big"
	"path"
	"strings"
	"time"

	"github.com/multiformats/go-multiaddr"

	"github.com/0xPolygon/polygon-edge/chain"
	"github.com/0xPolygon/polygon-edge/command"
	"github.com/0xPolygon/polygon-edge/command/helper"
	"github.com/0xPolygon/polygon-edge/consensus/polybft"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi/artifact"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/0xPolygon/polygon-edge/server"
	"github.com/0xPolygon/polygon-edge/types"
)

const (
	sprintSizeFlag = "sprint-size"
	blockTimeFlag  = "block-time"
	trieRootFlag   = "trieroot"

	blockTimeDriftFlag = "block-time-drift"

	defaultEpochSize                = uint64(10)
	defaultSprintSize               = uint64(5)
	defaultValidatorSetSize         = 100
	defaultBlockTime                = 2 * time.Second
	defaultEpochReward              = 1
	defaultBlockTimeDrift           = uint64(10)
	defaultBlockTrackerPollInterval = time.Second

	contractDeployerAllowListAdminFlag   = "contract-deployer-allow-list-admin"
	contractDeployerAllowListEnabledFlag = "contract-deployer-allow-list-enabled"
	contractDeployerBlockListAdminFlag   = "contract-deployer-block-list-admin"
	contractDeployerBlockListEnabledFlag = "contract-deployer-block-list-enabled"
	transactionsAllowListAdminFlag       = "transactions-allow-list-admin"
	transactionsAllowListEnabledFlag     = "transactions-allow-list-enabled"
	transactionsBlockListAdminFlag       = "transactions-block-list-admin"
	transactionsBlockListEnabledFlag     = "transactions-block-list-enabled"
	bridgeAllowListAdminFlag             = "bridge-allow-list-admin"
	bridgeAllowListEnabledFlag           = "bridge-allow-list-enabled"
	bridgeBlockListAdminFlag             = "bridge-block-list-admin"
	bridgeBlockListEnabledFlag           = "bridge-block-list-enabled"

	bootnodePortStart = 30301

	ecdsaAddressLength = 40
	blsKeyLength       = 256
	blsSignatureLength = 128
)

var (
	errNoGenesisValidators = errors.New("genesis validators aren't provided")
	errNoPremineAllowed    = errors.New("native token is not mintable, so no premine is allowed " +
		"except for zero address and reward wallet if native token is used as reward token")
)

type contractInfo struct {
	artifact *artifact.Artifact
	address  types.Address
}

type PricesDataCoinGecko struct {
	Prices [][]float64 `json:"prices"`
}

// generatePolyBftChainConfig creates and persists polybft chain configuration to the provided file path
func (p *genesisParams) generatePolyBftChainConfig(o command.OutputFormatter) error {
	// populate premine balance map
	premineBalances := make(map[types.Address]*helper.PremineInfo, len(p.premine))

	for _, premine := range p.premineInfos {
		premineBalances[premine.Address] = premine
	}

	if !p.nativeTokenConfig.IsMintable {
		// validate premine map, no premine is allowed if token is not mintable,
		// except for the zero address
		for a := range premineBalances {
			if a != types.ZeroAddress {
				return errNoPremineAllowed
			}
		}
	}

	initialValidators, err := p.getValidatorAccounts()
	if err != nil {
		return fmt.Errorf("failed to retrieve genesis validators: %w", err)
	}

	if len(initialValidators) == 0 {
		return errNoGenesisValidators
	}

	if _, err := o.Write([]byte("[GENESIS VALIDATORS]\n")); err != nil {
		return err
	}

	for _, v := range initialValidators {
		if _, err := o.Write([]byte(fmt.Sprintf("%v\n", v))); err != nil {
			return err
		}
	}

	polyBftConfig := &polybft.PolyBFTConfig{
		InitialValidatorSet: initialValidators,
		BlockTime:           common.Duration{Duration: p.blockTime},
		EpochSize:           p.epochSize,
		SprintSize:          p.sprintSize,
		EpochReward:         p.epochReward,
		// use 1st account as governance address
		Governance:               initialValidators[0].Address,
		InitialTrieRoot:          types.StringToHash(p.initialStateRoot),
		NativeTokenConfig:        p.nativeTokenConfig,
		MinValidatorSetSize:      p.minNumValidators,
		MaxValidatorSetSize:      p.maxNumValidators,
		BlockTimeDrift:           p.blockTimeDrift,
		BlockTrackerPollInterval: common.Duration{Duration: p.blockTrackerPollInterval},
		ProxyContractsAdmin:      types.StringToAddress(p.proxyContractsAdmin),
	}

	polyBftConfig.InitialPrices, err = getInitialPrices()
	if err != nil {
		return err
	}

	// Disable london hardfork if burn contract address is not provided
	enabledForks := chain.AllForksEnabled
	// Hydra modification: london hardfork is enabled no matter the burn contract state
	// if !p.isBurnContractEnabled() {
	// 	enabledForks.RemoveFork(chain.London)
	// }

	chainConfig := &chain.Chain{
		Name: p.name,
		Params: &chain.Params{
			ChainID: int64(p.chainID),
			Forks:   enabledForks,
			Engine: map[string]interface{}{
				string(server.PolyBFTConsensus): polyBftConfig,
			},
		},
		Bootnodes: p.bootnodes,
	}

	burnContractAddr := types.ZeroAddress

	if p.isBurnContractEnabled() {
		chainConfig.Params.BurnContract = make(map[uint64]types.Address, 1)

		burnContractInfo, err := parseBurnContractInfo(p.burnContract)
		if err != nil {
			return err
		}

		if !p.nativeTokenConfig.IsMintable {
			// burn contract can be specified on arbitrary address for non-mintable native tokens
			burnContractAddr = burnContractInfo.Address
			chainConfig.Params.BurnContract[burnContractInfo.BlockNumber] = burnContractAddr
			chainConfig.Params.BurnContractDestinationAddress = burnContractInfo.DestinationAddress
		} else {
			// burnt funds are sent to zero address when dealing with mintable native tokens
			chainConfig.Params.BurnContract[burnContractInfo.BlockNumber] = types.ZeroAddress
		}
	}

	totalStake := big.NewInt(0)

	for _, validator := range initialValidators {
		// increment total stake
		totalStake.Add(totalStake, validator.Stake)

		premineBalances[validator.Address] = &helper.PremineInfo{
			Address: validator.Address,
			Amount:  validator.Stake,
		}
	}

	// deploy genesis contracts
	allocs, err := p.deployContracts(totalStake)
	if err != nil {
		return err
	}

	// premine other accounts
	for _, premine := range premineBalances {
		// validators have already been premined, so no need to premine them again
		if _, ok := allocs[premine.Address]; ok {
			continue
		}

		allocs[premine.Address] = &chain.GenesisAccount{
			Balance: premine.Amount,
		}
	}

	validatorMetadata := make([]*validator.ValidatorMetadata, len(initialValidators))

	for i, validator := range initialValidators {
		// create validator metadata instance
		metadata, err := validator.ToValidatorMetadata(command.DefaultNumerator, polybft.DefaultDenominator)
		if err != nil {
			return err
		}

		validatorMetadata[i] = metadata

		// set genesis validators as boot nodes if boot nodes not provided via CLI
		if len(p.bootnodes) == 0 {
			chainConfig.Bootnodes = append(chainConfig.Bootnodes, validator.MultiAddr)
		}
	}

	genesisExtraData, err := GenerateExtraDataPolyBft(validatorMetadata)
	if err != nil {
		return err
	}

	// populate genesis parameters
	chainConfig.Genesis = &chain.Genesis{
		GasLimit:   p.blockGasLimit,
		Difficulty: 0,
		Alloc:      allocs,
		ExtraData:  genesisExtraData,
		GasUsed:    command.DefaultGenesisGasUsed,
		Mixhash:    polybft.HydragonMixDigest,
	}

	if len(p.contractDeployerAllowListAdmin) != 0 {
		// only enable allow list if there is at least one address as **admin**, otherwise
		// the allow list could never be updated
		chainConfig.Params.ContractDeployerAllowList = &chain.AddressListConfig{
			AdminAddresses:   stringSliceToAddressSlice(p.contractDeployerAllowListAdmin),
			EnabledAddresses: stringSliceToAddressSlice(p.contractDeployerAllowListEnabled),
		}
	}

	if len(p.contractDeployerBlockListAdmin) != 0 {
		// only enable block list if there is at least one address as **admin**, otherwise
		// the block list could never be updated
		chainConfig.Params.ContractDeployerBlockList = &chain.AddressListConfig{
			AdminAddresses:   stringSliceToAddressSlice(p.contractDeployerBlockListAdmin),
			EnabledAddresses: stringSliceToAddressSlice(p.contractDeployerBlockListEnabled),
		}
	}

	if len(p.transactionsAllowListAdmin) != 0 {
		// only enable allow list if there is at least one address as **admin**, otherwise
		// the allow list could never be updated
		chainConfig.Params.TransactionsAllowList = &chain.AddressListConfig{
			AdminAddresses:   stringSliceToAddressSlice(p.transactionsAllowListAdmin),
			EnabledAddresses: stringSliceToAddressSlice(p.transactionsAllowListEnabled),
		}
	}

	if len(p.transactionsBlockListAdmin) != 0 {
		// only enable block list if there is at least one address as **admin**, otherwise
		// the block list could never be updated
		chainConfig.Params.TransactionsBlockList = &chain.AddressListConfig{
			AdminAddresses:   stringSliceToAddressSlice(p.transactionsBlockListAdmin),
			EnabledAddresses: stringSliceToAddressSlice(p.transactionsBlockListEnabled),
		}
	}

	if len(p.bridgeAllowListAdmin) != 0 {
		// only enable allow list if there is at least one address as **admin**, otherwise
		// the allow list could never be updated
		chainConfig.Params.BridgeAllowList = &chain.AddressListConfig{
			AdminAddresses:   stringSliceToAddressSlice(p.bridgeAllowListAdmin),
			EnabledAddresses: stringSliceToAddressSlice(p.bridgeAllowListEnabled),
		}
	}

	if len(p.bridgeBlockListAdmin) != 0 {
		// only enable block list if there is at least one address as **admin**, otherwise
		// the block list could never be updated
		chainConfig.Params.BridgeBlockList = &chain.AddressListConfig{
			AdminAddresses:   stringSliceToAddressSlice(p.bridgeBlockListAdmin),
			EnabledAddresses: stringSliceToAddressSlice(p.bridgeBlockListEnabled),
		}
	}

	chainConfig.Genesis.BaseFee = p.parsedBaseFeeConfig.baseFee
	chainConfig.Genesis.BaseFeeEM = p.parsedBaseFeeConfig.baseFeeEM
	chainConfig.Genesis.BaseFeeChangeDenom = p.parsedBaseFeeConfig.baseFeeChangeDenom

	return helper.WriteGenesisConfigToDisk(chainConfig, params.genesisPath)
}

func (p *genesisParams) deployContracts(totalStake *big.Int) (map[types.Address]*chain.GenesisAccount, error) {
	proxyToImplAddrMap := contracts.GetProxyImplementationMapping()
	proxyAddresses := make([]types.Address, 0, len(proxyToImplAddrMap))

	for proxyAddr := range proxyToImplAddrMap {
		proxyAddresses = append(proxyAddresses, proxyAddr)
	}

	genesisContracts := []*contractInfo{
		{
			// BLS contract
			artifact: contractsapi.BLS,
			address:  contracts.BLSContractV1,
		},
		{
			artifact: contractsapi.HydraChain,
			address:  contracts.HydraChainContractV1,
		},
		{
			artifact: contractsapi.HydraStaking,
			address:  contracts.HydraStakingContractV1,
		},
		{
			artifact: contractsapi.HydraDelegation,
			address:  contracts.HydraDelegationContractV1,
		},
		{
			artifact: contractsapi.VestingManagerFactory,
			address:  contracts.VestingManagerFactoryContractV1,
		},
		{
			artifact: contractsapi.APRCalculator,
			address:  contracts.APRCalculatorContractV1,
		},
		{
			artifact: contractsapi.LiquidityToken,
			address:  contracts.LiquidityTokenContract,
		},
		{
			artifact: contractsapi.RewardWallet,
			address:  contracts.RewardWalletContractV1,
		},
		{
			// FeeHandler is an instance of the HydraVault contract
			artifact: contractsapi.HydraVault,
			address:  contracts.FeeHandlerContractV1,
		},
		{
			// DAOIncentiveVault is an instance of the HydraVault contract
			artifact: contractsapi.HydraVault,
			address:  contracts.DAOIncentiveVaultContractV1,
		},
		{
			artifact: contractsapi.PriceOracle,
			address:  contracts.PriceOracleContractV1,
		},
	}

	// if !params.nativeTokenConfig.IsMintable {
	// 	genesisContracts = append(genesisContracts,
	// 		&contractInfo{
	// 			artifact: contractsapi.NativeERC20,
	// 			address:  contracts.NativeERC20TokenContractV1,
	// 		})

	// 	// burn contract can be set only for non-mintable native token. If burn contract is set,
	// 	// default EIP1559 contract will be deployed.
	// 	if p.isBurnContractEnabled() {
	// 		genesisContracts = append(genesisContracts,
	// 			&contractInfo{
	// 				artifact: contractsapi.EIP1559Burn,
	// 				address:  burnContractAddr,
	// 			})

	// 		proxyAddresses = append(proxyAddresses, contracts.DefaultBurnContract)
	// 	}
	// } else {
	// 	genesisContracts = append(genesisContracts,
	// 		&contractInfo{
	// 			artifact: contractsapi.NativeERC20Mintable,
	// 			address:  contracts.NativeERC20TokenContractV1,
	// 		})
	// }

	allocations := make(map[types.Address]*chain.GenesisAccount, len(genesisContracts)+1)

	genesisContracts = append(genesisContracts, getProxyContractsInfo(proxyAddresses)...)

	for _, contract := range genesisContracts {
		allocations[contract.address] = &chain.GenesisAccount{
			Balance: big.NewInt(0),
			Code:    contract.artifact.DeployedBytecode,
		}
	}

	// HydraStaking must have funds pre-allocated, because of withdrawal workflow
	allocations[contracts.HydraStakingContract].Balance = totalStake

	// RewardWallet must have funds pre-allocated (2/3 of maxUint256)
	allocations[contracts.RewardWalletContract].Balance = common.GetTwoThirdOfMaxUint256()

	return allocations, nil
}

// getValidatorAccounts gathers validator accounts info either from CLI or from provided local storage
func (p *genesisParams) getValidatorAccounts() ([]*validator.GenesisValidator, error) {
	// populate validators premine info
	if len(p.validators) > 0 {
		validators := make([]*validator.GenesisValidator, len(p.validators))
		for i, val := range p.validators {
			parts := strings.Split(val, ":")
			if len(parts) != 4 {
				return nil, fmt.Errorf("expected 4 parts provided in the following format "+
					"<P2P multi address:ECDSA address:public BLS key:BLS signature>, but got %d part(s)",
					len(parts))
			}

			if _, err := multiaddr.NewMultiaddr(parts[0]); err != nil {
				return nil, fmt.Errorf("invalid P2P multi address '%s' provided: %w ", parts[0], err)
			}

			trimmedAddress := strings.TrimPrefix(parts[1], "0x")
			if len(trimmedAddress) != ecdsaAddressLength {
				return nil, fmt.Errorf("invalid ECDSA address: %s", parts[1])
			}

			trimmedBLSKey := strings.TrimPrefix(parts[2], "0x")
			if len(trimmedBLSKey) != blsKeyLength {
				return nil, fmt.Errorf("invalid BLS key: %s", parts[2])
			}

			if len(parts[3]) != blsSignatureLength {
				return nil, fmt.Errorf("invalid BLS signature: %s", parts[3])
			}

			addr := types.StringToAddress(trimmedAddress)
			validators[i] = &validator.GenesisValidator{
				MultiAddr: parts[0],
				Address:   addr,
				BlsKey:    trimmedBLSKey,
				Stake:     command.DefaultStake,
			}
		}

		return validators, nil
	}

	validatorsPath := p.validatorsPath
	if validatorsPath == "" {
		validatorsPath = path.Dir(p.genesisPath)
	}

	validators, err := ReadValidatorsByPrefix(validatorsPath, p.validatorsPrefixPath)
	if err != nil {
		return nil, err
	}

	for _, v := range validators {
		v.Stake = command.DefaultStake
	}

	return validators, nil
}

func stringSliceToAddressSlice(addrs []string) []types.Address {
	res := make([]types.Address, len(addrs))
	for indx, addr := range addrs {
		res[indx] = types.StringToAddress(addr)
	}

	return res
}

func getProxyContractsInfo(addresses []types.Address) []*contractInfo {
	result := make([]*contractInfo, len(addresses))

	for i, proxyAddress := range addresses {
		result[i] = &contractInfo{
			artifact: contractsapi.GenesisProxy,
			address:  proxyAddress,
		}
	}

	return result
}

// getInitialPrices returns an array of 310 *big.Int representing the initial prices to be used in the genesis.
// The prices are retrieved from a third party API and converted to *big.Int with 18 decimal places.
// If there is an error retrieving or converting the prices, the function returns the error.
func getInitialPrices() ([310]*big.Int, error) {
	convertedPrices := [310]*big.Int{}

	// get the prices data and populate the initial prices in the genesis
	priceData, err := getCGPricesData(8)
	if err != nil {
		return convertedPrices, err
	}

	for i, price := range priceData.Prices {
		convertedPrices[i], err = common.ConvertFloatToBigInt(price[1], 8)
		if err != nil {
			return convertedPrices, err
		}
	}

	return convertedPrices, nil
}
