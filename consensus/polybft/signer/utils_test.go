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

	expected := "27dc197e3b3633ec47f5f9dde9a3e63fbb1117fcba189bc408593e015af2a6b92c1d3699d5a05947d3eb96241a7b9fcab34d491134e32c02663774e3265c3f77"
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
