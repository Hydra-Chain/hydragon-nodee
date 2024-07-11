package polybft

import (
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

var (
	// bucket to store hydra chain state
	hydraChainStateBucket = []byte("hydraChainStateBucket")
	// key of the hydra chain state in bucket
	hydraChainStateKey = []byte("hydraChainState")
	// error returned if hydra chain state does not exists in db
	errNoHydraChainState = errors.New("hydra chain state not in db")
)

type StakeStore struct {
	db *bolt.DB
}

// initialize creates necessary buckets in DB if they don't already exist
func (s *StakeStore) initialize(tx *bolt.Tx) error {
	if _, err := tx.CreateBucketIfNotExists(hydraChainStateBucket); err != nil {
		return fmt.Errorf("failed to create bucket=%s: %w", string(epochsBucket), err)
	}

	return nil
}

// insertHydraChainState inserts the hydra chain state its bucket (or updates it if exists)
// If the passed tx is already open (not nil), it will use it to insert the hydra chain state
// If the passed tx is not open (it is nil), it will open a new transaction on db and insert hydra chain state
func (s *StakeStore) insertHydraChainState(hydraChainState HydraChainState, dbTx *bolt.Tx) error {
	insertFn := func(tx *bolt.Tx) error {
		raw, err := hydraChainState.Marshal()
		if err != nil {
			return err
		}

		return tx.Bucket(hydraChainStateBucket).Put(hydraChainStateKey, raw)
	}

	if dbTx == nil {
		return s.db.Update(func(tx *bolt.Tx) error {
			return insertFn(tx)
		})
	}

	return insertFn(dbTx)
}

// getHydraChainState returns the hydra chain state that contains the full list of validators, if exists
// If the passed tx is already open (not nil), it will use it to get all the validators within the hydra chain state
// If the passed tx is not open (it is nil), it will open a new transaction on db and get all the validators within the hydra chain state
func (s *StakeStore) getHydraChainState(dbTx *bolt.Tx) (HydraChainState, error) {
	var (
		hydraChainState HydraChainState
		err             error
	)

	getFn := func(tx *bolt.Tx) error {
		raw := tx.Bucket(hydraChainStateBucket).Get(hydraChainStateKey)
		if raw == nil {
			return errNoHydraChainState
		}

		return hydraChainState.Unmarshal(raw)
	}

	if dbTx == nil {
		err = s.db.View(func(tx *bolt.Tx) error {
			return getFn(tx)
		})
	} else {
		err = getFn(dbTx)
	}

	return hydraChainState, err
}
