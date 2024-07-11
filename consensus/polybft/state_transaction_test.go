package polybft

import (
	"encoding/hex"
	"math/big"
	"reflect"
	"testing"

	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/ethgo/abi"
)

func TestStateTransaction_Signature(t *testing.T) {
	t.Parallel()

	cases := []struct {
		m   *abi.Method
		sig string
	}{
		{
			contractsapi.HydraChain.Abi.GetMethod("commitEpoch"),
			"dab567de",
		},
	}
	for _, c := range cases {
		sig := hex.EncodeToString(c.m.ID())
		require.Equal(t, c.sig, sig)
	}
}

func TestStateTransaction_Encoding(t *testing.T) {
	t.Parallel()

	validators := validator.NewTestValidators(t, 5)
	validatorSet := validators.GetPublicIdentities()

	epochSize := int64(10)

	uptime := generateValidatorsUpTime(t, validatorSet, uint64(epochSize))

	cases := []contractsapi.StateTransactionInput{
		&contractsapi.CommitEpochHydraChainFn{
			ID: big.NewInt(1),
			Epoch: &contractsapi.Epoch{
				StartBlock: big.NewInt(1),
				EndBlock:   big.NewInt(epochSize),
				EpochRoot:  types.Hash{},
			},
			EpochSize: big.NewInt(epochSize),
			Uptime:    uptime,
		},
	}

	for _, c := range cases {
		res, err := c.EncodeAbi()

		require.NoError(t, err)

		// use reflection to create another type and decode
		val := reflect.New(reflect.TypeOf(c).Elem()).Interface()
		obj, ok := val.(contractsapi.StateTransactionInput)
		assert.True(t, ok)

		err = obj.DecodeAbi(res)
		require.NoError(t, err)

		require.Equal(t, obj, c)
	}
}
