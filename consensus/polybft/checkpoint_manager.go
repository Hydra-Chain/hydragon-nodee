package polybft

import (
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/umbracle/ethgo"
	bolt "go.etcd.io/bbolt"
)

type CheckpointManager interface {
	EventSubscriber
	PostBlock(req *PostBlockRequest) error
	BuildEventRoot(epoch uint64) (types.Hash, error)
	GenerateExitProof(exitID uint64) (types.Proof, error)
}

var _ CheckpointManager = (*dummyCheckpointManager)(nil)

type dummyCheckpointManager struct{}

func (d *dummyCheckpointManager) PostBlock(req *PostBlockRequest) error { return nil }
func (d *dummyCheckpointManager) BuildEventRoot(epoch uint64) (types.Hash, error) {
	return types.ZeroHash, nil
}
func (d *dummyCheckpointManager) GenerateExitProof(exitID uint64) (types.Proof, error) {
	return types.Proof{}, nil
}

// EventSubscriber implementation
func (d *dummyCheckpointManager) GetLogFilters() map[types.Address][]types.Hash {
	return make(map[types.Address][]types.Hash)
}
func (d *dummyCheckpointManager) ProcessLog(header *types.Header,
	log *ethgo.Log, dbTx *bolt.Tx) error {
	return nil
}
