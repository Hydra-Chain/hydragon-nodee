package contractsapi

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/require"
)

type method interface {
	EncodeAbi() ([]byte, error)
	DecodeAbi(buf []byte) error
}

func TestEncoding_Method(t *testing.T) {
	t.Parallel()

	epochSize := int64(10)

	uptime := make([]*Uptime, 5)

	for i := 0; i < 5; i++ {
		uptime[i] = &Uptime{
			Validator:    types.Address{},
			SignedBlocks: new(big.Int).SetUint64(uint64(epochSize)),
		}
	}

	cases := []method{
		// empty commit epoch
		&CommitEpochHydraChainFn{
			ID: big.NewInt(1),
			Epoch: &Epoch{
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
		obj, ok := val.(method)
		require.True(t, ok)

		err = obj.DecodeAbi(res)
		require.NoError(t, err)
		require.Equal(t, obj, c)
	}
}

func TestEncoding_Struct(t *testing.T) {
	t.Parallel()

	commitment := &StateSyncCommitment{
		StartID: big.NewInt(1),
		EndID:   big.NewInt(10),
		Root:    types.StringToHash("hash"),
	}

	encoding, err := commitment.EncodeAbi()
	require.NoError(t, err)

	var commitmentDecoded StateSyncCommitment

	require.NoError(t, commitmentDecoded.DecodeAbi(encoding))
	require.Equal(t, commitment.StartID, commitmentDecoded.StartID)
	require.Equal(t, commitment.EndID, commitmentDecoded.EndID)
	require.Equal(t, commitment.Root, commitmentDecoded.Root)
}
