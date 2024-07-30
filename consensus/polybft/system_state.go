package polybft

import (
	"fmt"
	"math/big"

	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/contract"
)

// BigNumDecimal is a data transfer object which holds bigNumbers that consist of numerator and denominator
type BigNumDecimal struct {
	Numerator   *big.Int
	Denominator *big.Int
}

// ValidatorInfo is data transfer object which holds validator information,
// provided by smart contract
type ValidatorInfo struct {
	Address             ethgo.Address `json:"address"`
	Stake               *big.Int      `json:"stake"`
	WithdrawableRewards *big.Int      `json:"withdrawableRewards"`
	IsActive            bool          `json:"isActive"`
	IsWhitelisted       bool          `json:"isWhitelisted"`
}

// SystemState is an interface to interact with the consensus system contracts in the chain
type SystemState interface {
	// GetEpoch retrieves current epoch number from the smart contract
	GetEpoch() (uint64, error)
	// GetNextCommittedIndex retrieves next committed bridge state sync index
	GetNextCommittedIndex() (uint64, error)
	// GetVotingPowerExponent retrieves voting power exponent from the HydraChain smart contract
	GetVotingPowerExponent() (exponent *BigNumDecimal, err error)
	// GetValidatorBlsKey retrieves validator BLS public key from the HydraChain smart contract
	GetValidatorBlsKey(addr types.Address) (*bls.PublicKey, error)
}

var _ SystemState = &SystemStateImpl{}

// SystemStateImpl is implementation of SystemState interface
type SystemStateImpl struct {
	hydraChainContract            *contract.Contract
	hydraStakingContract          *contract.Contract
	hydraDelegationContract       *contract.Contract
	vestingManagerFactoryContract *contract.Contract
	aprCalculatorContract         *contract.Contract
	rewardWalletContract          *contract.Contract
	sidechainBridgeContract       *contract.Contract
}

// NewSystemState initializes new instance of systemState which abstracts smart contracts functions
func NewSystemState(
	hydraChainAddr types.Address,
	hydraStakingAddr types.Address,
	hydraDelegationAddr types.Address,
	vestingManagerFactoryAddr types.Address,
	aprCalculatorAddr types.Address,
	rewardWalletAddr types.Address,
	stateRcvAddr types.Address,
	provider contract.Provider,
) *SystemStateImpl {
	s := &SystemStateImpl{}
	s.hydraChainContract = contract.NewContract(
		ethgo.Address(hydraChainAddr),
		contractsapi.HydraChain.Abi, contract.WithProvider(provider),
	)

	s.hydraStakingContract = contract.NewContract(
		ethgo.Address(hydraStakingAddr),
		contractsapi.HydraStaking.Abi, contract.WithProvider(provider),
	)

	s.hydraDelegationContract = contract.NewContract(
		ethgo.Address(hydraDelegationAddr),
		contractsapi.HydraDelegation.Abi, contract.WithProvider(provider),
	)

	s.vestingManagerFactoryContract = contract.NewContract(
		ethgo.Address(vestingManagerFactoryAddr),
		contractsapi.VestingManagerFactory.Abi, contract.WithProvider(provider),
	)

	s.aprCalculatorContract = contract.NewContract(
		ethgo.Address(aprCalculatorAddr),
		contractsapi.APRCalculator.Abi, contract.WithProvider(provider),
	)

	s.rewardWalletContract = contract.NewContract(
		ethgo.Address(rewardWalletAddr),
		contractsapi.RewardWallet.Abi, contract.WithProvider(provider),
	)

	// Hydra modification: StateReceiver contract is not used
	// s.sidechainBridgeContract = contract.NewContract(
	// 	ethgo.Address(stateRcvAddr),
	// 	contractsapi.StateReceiver.Abi,
	// 	contract.WithProvider(provider),
	// )

	return s
}

// GetEpoch retrieves current epoch number from the smart contract
func (s *SystemStateImpl) GetEpoch() (uint64, error) {
	rawResult, err := s.hydraChainContract.Call("currentEpochId", ethgo.Latest)
	if err != nil {
		return 0, err
	}

	epochNumber, isOk := rawResult["0"].(*big.Int)
	if !isOk {
		return 0, fmt.Errorf("failed to decode epoch")
	}

	return epochNumber.Uint64(), nil
}

// H: add a function to fetch the voting power exponent
func (s *SystemStateImpl) GetVotingPowerExponent() (exponent *BigNumDecimal, err error) {
	rawOutput, err := s.hydraChainContract.Call("getExponent", ethgo.Latest)
	if err != nil {
		return nil, err
	}

	expNumerator, ok := rawOutput["numerator"].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("failed to decode voting power exponent numerator")
	}

	expDenominator, ok := rawOutput["denominator"].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("failed to decode voting power exponent denominator")
	}

	return &BigNumDecimal{Numerator: expNumerator, Denominator: expDenominator}, nil
}

// H: add a function to fetch the validator bls key
func (s *SystemStateImpl) GetValidatorBlsKey(addr types.Address) (*bls.PublicKey, error) {
	rawOutput, err := s.hydraChainContract.Call("getValidator", ethgo.Latest, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to call getValidator function: %w", err)
	}

	rawKey, ok := rawOutput["blsKey"].([4]*big.Int)
	if !ok {
		return nil, fmt.Errorf("failed to decode blskey")
	}

	blsKey, err := bls.UnmarshalPublicKeyFromBigInt(rawKey)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal BLS public key: %w", err)
	}

	return blsKey, nil
}

// GetNextCommittedIndex retrieves next committed bridge state sync index
func (s *SystemStateImpl) GetNextCommittedIndex() (uint64, error) {
	rawResult, err := s.sidechainBridgeContract.Call("lastCommittedId", ethgo.Latest)
	if err != nil {
		return 0, err
	}

	nextCommittedIndex, isOk := rawResult["0"].(*big.Int)
	if !isOk {
		return 0, fmt.Errorf("failed to decode next committed index")
	}

	return nextCommittedIndex.Uint64() + 1, nil
}
