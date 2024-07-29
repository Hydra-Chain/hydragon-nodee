package validator

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/0xPolygon/polygon-edge/types"
)

// GenesisValidator represents public information about validator accounts which are the part of genesis
type GenesisValidator struct {
	Address       types.Address
	BlsPrivateKey *bls.PrivateKey
	BlsKey        string
	BlsSignature  string
	Stake         *big.Int
	MultiAddr     string
}

type genesisValidatorRaw struct {
	Address      types.Address `json:"address"`
	BlsKey       string        `json:"blsKey"`
	BlsSignature string        `json:"blsSignature"`
	Stake        *string       `json:"stake"`
	MultiAddr    string        `json:"multiAddr"`
}

func (v *GenesisValidator) MarshalJSON() ([]byte, error) {
	raw := &genesisValidatorRaw{Address: v.Address, BlsKey: v.BlsKey, MultiAddr: v.MultiAddr, BlsSignature: v.BlsSignature}
	raw.Stake = common.EncodeBigInt(v.Stake)

	return json.Marshal(raw)
}

func (v *GenesisValidator) UnmarshalJSON(data []byte) (err error) {
	var raw genesisValidatorRaw

	if err = json.Unmarshal(data, &raw); err != nil {
		return err
	}

	v.Address = raw.Address
	v.BlsKey = raw.BlsKey
	v.BlsSignature = raw.BlsSignature
	v.MultiAddr = raw.MultiAddr

	v.Stake, err = common.ParseUint256orHex(raw.Stake)

	return err
}

// UnmarshalBLSPublicKey unmarshals the hex encoded BLS public key
func (v *GenesisValidator) UnmarshalBLSPublicKey() (*bls.PublicKey, error) {
	decoded, err := hex.DecodeString(v.BlsKey)
	if err != nil {
		return nil, err
	}

	return bls.UnmarshalPublicKey(decoded)
}

// UnmarshalBLSSignature unmarshals the hex encoded BLS signature
func (v *GenesisValidator) UnmarshalBLSSignature() (*bls.Signature, error) {
	decoded, err := hex.DecodeString(v.BlsSignature)
	if err != nil {
		return nil, err
	}

	return bls.UnmarshalSignature(decoded)
}

// ToValidatorInitAPIBinding converts GenesisValidator to instance of contractsapi.ValidatorInit
func (v GenesisValidator) ToValidatorInitAPIBinding() (*contractsapi.ValidatorInit, error) {
	blsSignature, err := v.UnmarshalBLSSignature()
	if err != nil {
		return nil, err
	}
	signBigInts, err := blsSignature.ToBigInt()
	if err != nil {
		return nil, err
	}
	pubKey, err := v.UnmarshalBLSPublicKey()
	if err != nil {
		return nil, err
	}
	return &contractsapi.ValidatorInit{
		Addr:      v.Address,
		Pubkey:    pubKey.ToBigInt(),
		Signature: signBigInts,
	}, nil
}

// ToStakerInitAPIBinding converts GenesisValidator to instance of contractsapi.StakerInit
func (v GenesisValidator) ToStakerInitAPIBinding() (*contractsapi.StakerInit, error) {
	return &contractsapi.StakerInit{
		Addr:  v.Address,
		Stake: new(big.Int).Set(v.Stake),
	}, nil
}

// GetInitialStakers converts the initial validators to array of contracts.StakerInit
func GetInitialStakers(initialValidators []*GenesisValidator) ([]*contractsapi.StakerInit, error) {
	initialStakers := make([]*contractsapi.StakerInit, len(initialValidators))

	for i, validator := range initialValidators {
		validatorData, err := validator.ToStakerInitAPIBinding()
		if err != nil {
			return nil, err
		}

		initialStakers[i] = validatorData
	}

	return initialStakers, nil
}

// ToValidatorMetadata creates ValidatorMetadata instance
func (v *GenesisValidator) ToValidatorMetadata(expNum *big.Int, expDen *big.Int) (*ValidatorMetadata, error) {
	blsKey, err := v.UnmarshalBLSPublicKey()
	if err != nil {
		return nil, err
	}

	vpower := CalculateVPower(v.Stake, expNum, expDen)
	fmt.Println("Validator metadata set", "address", v.Address, "stake is", v.Stake, "voting power is", vpower)

	metadata := &ValidatorMetadata{
		Address:     v.Address,
		BlsKey:      blsKey,
		VotingPower: vpower,
		IsActive:    true,
	}

	return metadata, nil
}

// String implements fmt.Stringer interface
func (v *GenesisValidator) String() string {
	return fmt.Sprintf("Address=%s; Stake=%d; P2P Multi addr=%s; BLS Key=%s;",
		v.Address, v.Stake, v.MultiAddr, v.BlsKey)
}
