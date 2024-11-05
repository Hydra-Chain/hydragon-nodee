package validator

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/bitmap"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateRandomBytes generates byte array with random data of 32 bytes length
func generateRandomBytes(t *testing.T) (result []byte) {
	t.Helper()

	result = make([]byte, types.HashLength)
	_, err := rand.Reader.Read(result)
	require.NoError(t, err, "Cannot generate random byte array content.")

	return
}

func TestValidatorMetadata_Equals(t *testing.T) {
	t.Parallel()

	v := NewTestValidator(t, "A", 10)
	validatorAcc := v.ValidatorMetadata()
	// proper validator metadata instance doesn't equal to nil
	require.False(t, validatorAcc.Equals(nil))
	// same instances of validator metadata are equal
	require.True(t, validatorAcc.Equals(v.ValidatorMetadata()))

	// update voting power => validator metadata instances aren't equal
	validatorAcc.VotingPower = new(big.Int).SetInt64(50)
	require.False(t, validatorAcc.Equals(v.ValidatorMetadata()))
}

func TestValidatorMetadata_CalculateVPower(t *testing.T) {
	t.Parallel()

	createInt := func(integer string) *big.Int {
		res, ok := new(big.Int).SetString(integer, 10)
		if !ok {
			panic("failed to create big.Int")
		}

		return res
	}

	cases := []struct {
		stakedBalance  string
		expNumerator   string
		expDenominator string
		result         string
	}{
		{"1", "8500", "10000", "0"},                                    // 0.000000000000000001 Coins
		{"999999999999999999", "8500", "10000", "0"},                   // 0.999999999999999999 Coins
		{"1000000000000000000", "8500", "10000", "1"},                  // 1 Coin
		{"2000000000000000000", "8500", "10000", "1"},                  // 2 Coins
		{"3000000000000000000", "8500", "10000", "2"},                  // 3 Coins
		{"4000000000000000000", "8500", "10000", "3"},                  // 4 Coins
		{"5000000000000000000", "8500", "10000", "3"},                  // 5 Coins
		{"6000000000000000000", "8500", "10000", "4"},                  // 6 Coins
		{"7000000000000000000", "8500", "10000", "5"},                  // 7 Coins
		{"10000000000000000000", "8500", "10000", "7"},                 // 10 Coins
		{"25000000000000000000", "8500", "10000", "15"},                // 25 Coins
		{"50000000000000000000", "8500", "10000", "27"},                // 50 Coins
		{"128324324324324324324", "8500", "10000", "61"},               // 128,324324324324324324 Coins
		{"300000000000000000000", "8500", "10000", "127"},              // 300 Coins
		{"750000000000000000000", "8500", "10000", "277"},              // 750 Coins
		{"1500000000000000000000", "8500", "10000", "500"},             // 1500 Coins
		{"3000000000000000000000", "8500", "10000", "902"},             // 3000 Coins
		{"6000000000000000000000", "8500", "10000", "1627"},            // 6000 Coins
		{"12000000000000000000000", "8500", "10000", "2932"},           // 12000 Coins
		{"24000000000000000000000", "8500", "10000", "5286"},           // 24000 Coins
		{"48000000000000000000000", "8500", "10000", "9529"},           // 48000 Coins
		{"96000000000000000000000", "8500", "10000", "17176"},          // 96000 Coins
		{"50000000000000000000000000", "8500", "10000", "3500455"},     // 50 000 000 Coins
		{"9900000000000000000000000", "8500", "10000", "883669"},       // 9 900 000 Coins
		{"9990000000000000000000000000", "8500", "10000", "315958952"}, // 9 900 900 000 Coins

		{"6000000000000000000000", "5000", "10000", "77"},          // 6000 Coins
		{"12000000000000000000000", "5000", "10000", "109"},        // 12000 Coins
		{"24000000000000000000000", "5000", "10000", "154"},        // 24000 Coins
		{"48000000000000000000000", "5000", "10000", "219"},        // 48000 Coins
		{"96000000000000000000000", "5000", "10000", "309"},        // 96000 Coins
		{"9900000000000000000000000", "5000", "10000", "3146"},     // 9 900 000 Coins
		{"50000000000000000000000000", "5000", "10000", "7071"},    // 50 000 000 Coins
		{"9990000000000000000000000000", "5000", "10000", "99949"}, // 9 900 900 000 Coins
	}

	for _, c := range cases {
		quorumSize := CalculateVPower(createInt(c.stakedBalance), createInt(c.expNumerator), createInt(c.expDenominator))
		require.Equal(t, createInt(c.result), quorumSize)
	}
}

func TestValidatorMetadata_EqualAddressAndBlsKey(t *testing.T) {
	t.Parallel()

	v := NewTestValidator(t, "A", 10)
	validatorAcc := v.ValidatorMetadata()
	// proper validator metadata instance doesn't equal to nil
	require.False(t, validatorAcc.EqualAddressAndBlsKey(nil))
	// same instances of validator metadata are equal
	require.True(t, validatorAcc.EqualAddressAndBlsKey(v.ValidatorMetadata()))

	// update voting power => validator metadata instances aren't equal
	validatorAcc.Address = types.BytesToAddress(generateRandomBytes(t))
	require.False(t, validatorAcc.EqualAddressAndBlsKey(v.ValidatorMetadata()))
}

func TestAccountSet_GetAddresses(t *testing.T) {
	t.Parallel()

	address1, address2, address3 := types.Address{4, 3}, types.Address{68, 123}, types.Address{168, 123}
	ac := AccountSet{
		&ValidatorMetadata{Address: address1},
		&ValidatorMetadata{Address: address2},
		&ValidatorMetadata{Address: address3},
	}
	rs := ac.GetAddresses()
	assert.Len(t, rs, 3)
	assert.Equal(t, address1, rs[0])
	assert.Equal(t, address2, rs[1])
	assert.Equal(t, address3, rs[2])
}

func TestAccountSet_GetBlsKeys(t *testing.T) {
	t.Parallel()

	keys, err := bls.CreateRandomBlsKeys(3)
	assert.NoError(t, err)

	key1, key2, key3 := keys[0], keys[1], keys[2]
	ac := AccountSet{
		&ValidatorMetadata{BlsKey: key1.PublicKey()},
		&ValidatorMetadata{BlsKey: key2.PublicKey()},
		&ValidatorMetadata{BlsKey: key3.PublicKey()},
	}
	rs := ac.GetBlsKeys()
	assert.Len(t, rs, 3)
	assert.Equal(t, key1.PublicKey(), rs[0])
	assert.Equal(t, key2.PublicKey(), rs[1])
	assert.Equal(t, key3.PublicKey(), rs[2])
}

func TestAccountSet_IndexContainsAddressesAndContainsNodeId(t *testing.T) {
	t.Parallel()

	const count = 10

	dummy := types.Address{2, 3, 4}
	validators := NewTestValidators(t, count).GetPublicIdentities()
	addresses := [count]types.Address{}

	for i, validator := range validators {
		addresses[i] = validator.Address
	}

	for i, a := range addresses {
		assert.Equal(t, i, validators.Index(a))
		assert.True(t, validators.ContainsAddress(a))
		assert.True(t, validators.ContainsNodeID(a.String()))
	}

	assert.Equal(t, -1, validators.Index(dummy))
	assert.False(t, validators.ContainsAddress(dummy))
	assert.False(t, validators.ContainsNodeID(dummy.String()))
}

func TestAccountSet_Len(t *testing.T) {
	t.Parallel()

	const count = 10

	ac := AccountSet{}

	for i := 0; i < count; i++ {
		ac = append(ac, &ValidatorMetadata{})
		assert.Equal(t, i+1, ac.Len())
	}
}

func TestAccountSet_ApplyDelta(t *testing.T) {
	t.Parallel()

	type Step struct {
		added    []string
		updated  map[string]uint64
		removed  []uint64
		expected map[string]uint64
		errMsg   string
	}

	cases := []struct {
		name  string
		steps []*Step
	}{
		{
			name: "Basic",
			steps: []*Step{
				{
					[]string{"A", "B", "C", "D"},
					nil,
					nil,
					map[string]uint64{"A": 15000, "B": 15000, "C": 15000, "D": 15000},
					"",
				},
				{
					// add two new validators and remove 3 (one does not exists)
					// update voting powers to subset of validators
					// (two of them added in the previous step and one added in the current one)
					[]string{"E", "F"},
					map[string]uint64{"A": 30, "D": 10, "E": 5},
					[]uint64{1, 2, 5},
					map[string]uint64{"A": 30, "D": 10, "E": 5, "F": 15000},
					"",
				},
			},
		},
		{
			name: "AddRemoveSameValidator",
			steps: []*Step{
				{
					[]string{"A"},
					nil,
					[]uint64{0},
					map[string]uint64{"A": 15000},
					"",
				},
			},
		},
		{
			name: "AddSameValidatorTwice",
			steps: []*Step{
				{
					[]string{"A", "A"},
					nil,
					nil,
					nil,
					"is already present in the validators snapshot",
				},
			},
		},
		{
			name: "UpdateNonExistingValidator",
			steps: []*Step{
				{
					nil,
					map[string]uint64{"B": 5},
					nil,
					nil,
					"incorrect delta provided: validator",
				},
			},
		},
	}

	for _, cc := range cases {
		cc := cc
		t.Run(cc.name, func(t *testing.T) {
			t.Parallel()

			snapshot := AccountSet{}
			// Add a couple of validators to the snapshot => validators are present in the snapshot after applying such delta
			vals := NewTestValidatorsWithAliases(t, []string{"A", "B", "C", "D", "E", "F"})

			for _, step := range cc.steps {
				addedValidators := AccountSet{}
				if step.added != nil {
					addedValidators = vals.GetPublicIdentities(step.added...)
				}

				delta := &ValidatorSetDelta{
					Added:   addedValidators,
					Removed: bitmap.Bitmap{},
				}

				for _, i := range step.removed {
					delta.Removed.Set(i)
				}

				// update voting powers
				delta.Updated = vals.UpdateVotingPowers(step.updated)

				// apply delta
				var err error

				snapshot, err = snapshot.ApplyDelta(delta)
				if step.errMsg != "" {
					require.ErrorContains(t, err, step.errMsg)
					require.Nil(t, snapshot)

					return
				}

				require.NoError(t, err)

				// validate validator set
				require.Equal(t, len(step.expected), snapshot.Len())

				for validatorAlias, votingPower := range step.expected {
					v := vals.GetValidator(validatorAlias).ValidatorMetadata()
					require.True(t, snapshot.ContainsAddress(v.Address), "validator '%s' not found in snapshot", validatorAlias)
					require.Equal(t, new(big.Int).SetUint64(votingPower), v.VotingPower)
				}
			}
		})
	}
}

func TestAccountSet_ApplyEmptyDelta(t *testing.T) {
	t.Parallel()

	v := NewTestValidatorsWithAliases(t, []string{"A", "B", "C", "D", "E", "F"})
	validatorAccs := v.GetPublicIdentities()
	validators, err := validatorAccs.ApplyDelta(nil)
	require.NoError(t, err)
	require.Equal(t, validatorAccs, validators)
}

func TestAccountSet_Hash(t *testing.T) {
	t.Parallel()

	t.Run("Hash non-empty account set", func(t *testing.T) {
		t.Parallel()

		v := NewTestValidatorsWithAliases(t, []string{"A", "B", "C", "D", "E", "F"})
		hash, err := v.GetPublicIdentities().Hash()
		require.NoError(t, err)
		require.NotEqual(t, types.ZeroHash, hash)
	})

	t.Run("Hash empty account set", func(t *testing.T) {
		t.Parallel()

		empty := AccountSet{}
		hash, err := empty.Hash()
		require.NoError(t, err)
		require.NotEqual(t, types.ZeroHash, hash)
	})
}
