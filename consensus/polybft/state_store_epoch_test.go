package polybft

import (
	"testing"

	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestState_insertAndGetValidatorSnapshot(t *testing.T) {
	t.Parallel()

	const (
		epoch            = uint64(1)
		epochEndingBlock = uint64(100)
	)

	state := newTestState(t)
	keys, err := bls.CreateRandomBlsKeys(3)

	require.NoError(t, err)

	snapshot := validator.AccountSet{
		&validator.ValidatorMetadata{Address: types.BytesToAddress([]byte{0x18}), BlsKey: keys[0].PublicKey()},
		&validator.ValidatorMetadata{Address: types.BytesToAddress([]byte{0x23}), BlsKey: keys[1].PublicKey()},
		&validator.ValidatorMetadata{Address: types.BytesToAddress([]byte{0x37}), BlsKey: keys[2].PublicKey()},
	}

	assert.NoError(t, state.EpochStore.insertValidatorSnapshot(
		&validatorSnapshot{epoch, epochEndingBlock, snapshot}, nil))

	snapshotFromDB, err := state.EpochStore.getValidatorSnapshot(epoch)

	assert.NoError(t, err)
	assert.Equal(t, snapshot.Len(), snapshotFromDB.Snapshot.Len())
	assert.Equal(t, epoch, snapshotFromDB.Epoch)
	assert.Equal(t, epochEndingBlock, snapshotFromDB.EpochEndingBlock)

	for i, v := range snapshot {
		assert.Equal(t, v.Address, snapshotFromDB.Snapshot[i].Address)
		assert.Equal(t, v.BlsKey, snapshotFromDB.Snapshot[i].BlsKey)
	}
}

func TestState_cleanValidatorSnapshotsFromDb(t *testing.T) {
	t.Parallel()

	fixedEpochSize := uint64(10)
	state := newTestState(t)
	keys, err := bls.CreateRandomBlsKeys(3)
	require.NoError(t, err)

	snapshot := validator.AccountSet{
		&validator.ValidatorMetadata{Address: types.BytesToAddress([]byte{0x18}), BlsKey: keys[0].PublicKey()},
		&validator.ValidatorMetadata{Address: types.BytesToAddress([]byte{0x23}), BlsKey: keys[1].PublicKey()},
		&validator.ValidatorMetadata{Address: types.BytesToAddress([]byte{0x37}), BlsKey: keys[2].PublicKey()},
	}

	var epoch uint64
	// add a couple of more snapshots above limit just to make sure we reached it
	for i := 1; i <= validatorSnapshotLimit+2; i++ {
		epoch = uint64(i)
		assert.NoError(t, state.EpochStore.insertValidatorSnapshot(
			&validatorSnapshot{epoch, epoch * fixedEpochSize, snapshot}, nil))
	}

	snapshotFromDB, err := state.EpochStore.getValidatorSnapshot(epoch)

	assert.NoError(t, err)
	assert.Equal(t, snapshot.Len(), snapshotFromDB.Snapshot.Len())
	assert.Equal(t, epoch, snapshotFromDB.Epoch)
	assert.Equal(t, epoch*fixedEpochSize, snapshotFromDB.EpochEndingBlock)

	for i, v := range snapshot {
		assert.Equal(t, v.Address, snapshotFromDB.Snapshot[i].Address)
		assert.Equal(t, v.BlsKey, snapshotFromDB.Snapshot[i].BlsKey)
	}

	assert.NoError(t, state.EpochStore.cleanValidatorSnapshotsFromDB(epoch, nil))

	// test that last (numberOfSnapshotsToLeaveInDb) of snapshots are left in db after cleanup
	validatorSnapshotsBucketStats, err := state.EpochStore.validatorSnapshotsDBStats()
	require.NoError(t, err)

	assert.Equal(t, numberOfSnapshotsToLeaveInDB, validatorSnapshotsBucketStats.KeyN)

	for i := 0; i < numberOfSnapshotsToLeaveInDB; i++ {
		snapshotFromDB, err = state.EpochStore.getValidatorSnapshot(epoch)
		assert.NoError(t, err)
		assert.NotNil(t, snapshotFromDB)
		epoch--
	}
}

func TestState_getLastSnapshot(t *testing.T) {
	t.Parallel()

	const (
		lastEpoch          = uint64(10)
		fixedEpochSize     = uint64(10)
		numberOfValidators = 3
	)

	state := newTestState(t)

	for i := uint64(1); i <= lastEpoch; i++ {
		keys, err := bls.CreateRandomBlsKeys(numberOfValidators)

		require.NoError(t, err)

		var snapshot validator.AccountSet
		for j := 0; j < numberOfValidators; j++ {
			snapshot = append(snapshot, &validator.ValidatorMetadata{Address: types.BytesToAddress(generateRandomBytes(t)), BlsKey: keys[j].PublicKey()})
		}

		require.NoError(t, state.EpochStore.insertValidatorSnapshot(
			&validatorSnapshot{i, i * fixedEpochSize, snapshot}, nil))
	}

	snapshotFromDB, err := state.EpochStore.getLastSnapshot(nil)

	assert.NoError(t, err)
	assert.Equal(t, numberOfValidators, snapshotFromDB.Snapshot.Len())
	assert.Equal(t, lastEpoch, snapshotFromDB.Epoch)
	assert.Equal(t, lastEpoch*fixedEpochSize, snapshotFromDB.EpochEndingBlock)
}
