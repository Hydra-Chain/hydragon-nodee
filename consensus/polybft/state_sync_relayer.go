package polybft

import (
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/umbracle/ethgo"
	bolt "go.etcd.io/bbolt"
)

// StateSyncRelayer is an interface that defines functions for state sync relayer
type StateSyncRelayer interface {
	EventSubscriber
	PostBlock(req *PostBlockRequest) error
	Init() error
	Close()
}

// stateSyncProofRetriever is an interface that exposes function for retrieving state sync proof
type stateSyncProofRetriever interface {
	GetStateSyncProof(stateSyncID uint64) (types.Proof, error)
}

var _ StateSyncRelayer = (*dummyStateSyncRelayer)(nil)

// dummyStateSyncRelayer is a dummy implementation of a StateSyncRelayer
type dummyStateSyncRelayer struct{}

func (d *dummyStateSyncRelayer) PostBlock(req *PostBlockRequest) error { return nil }

func (d *dummyStateSyncRelayer) Init() error { return nil }
func (d *dummyStateSyncRelayer) Close()      {}

// EventSubscriber implementation
func (d *dummyStateSyncRelayer) GetLogFilters() map[types.Address][]types.Hash {
	return make(map[types.Address][]types.Hash)
}
func (d *dummyStateSyncRelayer) ProcessLog(header *types.Header, log *ethgo.Log, dbTx *bolt.Tx) error {
	return nil
}
