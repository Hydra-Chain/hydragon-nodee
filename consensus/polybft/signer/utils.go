package signer

import (
	"math/big"

	"github.com/umbracle/ethgo/abi"

	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/0xPolygon/polygon-edge/crypto"
	"github.com/0xPolygon/polygon-edge/types"
)

var (
	addressABIType = abi.MustNewType("address")
	uint256ABIType = abi.MustNewType("uint256")
)

const (
	DomainValidatorSetString      = "DOMAIN_CHILD_VALIDATOR_SET"
	DomainCheckpointManagerString = "DOMAIN_CHECKPOINT_MANAGER"
	DomainCommonSigningString     = "DOMAIN_COMMON_SIGNING"
	DomainStateReceiverString     = "DOMAIN_STATE_RECEIVER"
)

var (
	// domain used to map hash to G1 used by (child) validator set
	DomainValidatorSet = crypto.Keccak256([]byte(DomainValidatorSetString))

	// domain used to map hash to G1 used by child checkpoint manager
	DomainCheckpointManager = crypto.Keccak256([]byte(DomainCheckpointManagerString))

	DomainCommonSigning = crypto.Keccak256([]byte(DomainCommonSigningString))
	DomainStateReceiver = crypto.Keccak256([]byte(DomainStateReceiverString))
)

// H_MODIFY: Generate KOSK signature without SupernetManager parameter (not L2 chain)
// MakeKOSKSignature creates KOSK signature which prevents rogue attack
func MakeKOSKSignature(
	privateKey *bls.PrivateKey, address types.Address, chainID int64, domain []byte) (*bls.Signature, error) {
	message, err := abi.Encode(
		[]interface{}{address, big.NewInt(chainID)},
		abi.MustNewType("tuple(address, uint256)"))
	if err != nil {
		return nil, err
	}

	// abi.Encode adds 12 zero bytes before actual address bytes
	return privateKey.Sign(message[12:], domain)
}
