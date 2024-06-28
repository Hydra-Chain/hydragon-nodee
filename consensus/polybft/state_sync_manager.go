package polybft

import (
	"github.com/umbracle/ethgo"
	bolt "go.etcd.io/bbolt"

	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/types"
)

type Runtime interface {
	IsActiveValidator() bool
}

type StateSyncProof struct {
	Proof     []types.Hash
	StateSync *contractsapi.StateSyncedEvent
}

// StateSyncManager is an interface that defines functions for state sync workflow
type StateSyncManager interface {
	EventSubscriber
	Init() error
	Close()
	Commitment(blockNumber uint64) (*CommitmentMessageSigned, error)
	GetStateSyncProof(stateSyncID uint64) (types.Proof, error)
	PostBlock(req *PostBlockRequest) error
	PostEpoch(req *PostEpochRequest) error
}

var _ StateSyncManager = (*dummyStateSyncManager)(nil)

// dummyStateSyncManager is used when bridge is not enabled
type dummyStateSyncManager struct{}

func (d *dummyStateSyncManager) Init() error { return nil }
func (d *dummyStateSyncManager) Close()      {}
func (d *dummyStateSyncManager) Commitment(blockNumber uint64) (*CommitmentMessageSigned, error) {
	return nil, nil
}
func (d *dummyStateSyncManager) PostBlock(req *PostBlockRequest) error { return nil }
func (d *dummyStateSyncManager) PostEpoch(req *PostEpochRequest) error { return nil }
func (d *dummyStateSyncManager) GetStateSyncProof(stateSyncID uint64) (types.Proof, error) {
	return types.Proof{}, nil
}

// EventSubscriber implementation
func (d *dummyStateSyncManager) GetLogFilters() map[types.Address][]types.Hash {
	return make(map[types.Address][]types.Hash)
}
func (d *dummyStateSyncManager) ProcessLog(header *types.Header,
	log *ethgo.Log, dbTx *bolt.Tx) error {
	return nil
}
