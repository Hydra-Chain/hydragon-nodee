package signer

import (
	"encoding/hex"
	"testing"

	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_MakeKOSKSignature(t *testing.T) {
	t.Parallel()

	expected := "14e0f68d2a01ee77f6ac43fc67984f464aed88ca20fc9da23ee89b388b63def51fbee740cd6dc8cfd8cd8c5dadf2d20772c337834833c9c905b8a0314810e5b4"
	bytes, _ := hex.DecodeString("3139343634393730313533353434353137333331343333303931343932303731313035313730303336303738373134363131303435323837383335373237343933383834303135343336383231")

	pk, err := bls.UnmarshalPrivateKey(bytes)
	require.NoError(t, err)

	address := types.BytesToAddress((pk.PublicKey().Marshal())[:types.AddressLength])

	signature, err := MakeKOSKSignature(pk, address, 10, DomainHydraChain)
	require.NoError(t, err)

	signatureBytes, err := signature.Marshal()
	require.NoError(t, err)

	assert.Equal(t, expected, hex.EncodeToString(signatureBytes))

	signature, err = MakeKOSKSignature(pk, address, 100, DomainHydraChain)
	require.NoError(t, err)

	signatureBytes, err = signature.Marshal()
	require.NoError(t, err)

	assert.NotEqual(t, expected, hex.EncodeToString(signatureBytes))
}
