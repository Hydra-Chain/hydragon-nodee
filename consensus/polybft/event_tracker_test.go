package polybft

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/testutil"
)

type mockEventSubscriber struct {
	lock sync.RWMutex
	logs []*ethgo.Log
}

func (m *mockEventSubscriber) AddLog(log *ethgo.Log) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if len(m.logs) == 0 {
		m.logs = []*ethgo.Log{}
	}

	m.logs = append(m.logs, log)
}

func (m *mockEventSubscriber) len() int {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return len(m.logs)
}

func TestEventTracker_TrackSyncEvents(t *testing.T) {
	t.Parallel()

	server := testutil.DeployTestServer(t, nil)

	tmpDir, err := os.MkdirTemp("/tmp", "test-event-tracker")
	defer os.RemoveAll(tmpDir)
	require.NoError(t, err)

	cc := &testutil.Contract{}
	cc.AddCallback(func() string {
		return `
			event StateSync(uint256 indexed id, address indexed target, bytes data);

			function emitEvent() public payable {
				emit StateSync(1, msg.sender, bytes(""));
			}
			`
	})

	_, addr, err := server.DeployContract(cc)
	require.NoError(t, err)

	// prefill with 10 events
	for i := 0; i < 10; i++ {
		receipt, err := server.TxnTo(addr, "emitEvent")
		require.NoError(t, err)
		require.Equal(t, uint64(types.ReceiptSuccess), receipt.Status)
	}

	sub := &mockEventSubscriber{}

	tracker := &eventTracker{
		logger:     hclog.NewNullLogger(),
		subscriber: sub,
		dataDir:    tmpDir,
		config: &PolyBFTConfig{
			Bridge: &BridgeConfig{
				JSONRPCEndpoint: server.HTTPAddr(),
				BridgeAddr:      types.Address(addr),
			},
		},
	}

	err = tracker.start()
	require.NoError(t, err)

	time.Sleep(2 * time.Second)
	require.Equal(t, sub.len(), 10)
	// send 10 more events
	for i := 0; i < 10; i++ {
		receipt, err := server.TxnTo(addr, "emitEvent")
		require.NoError(t, err)
		require.Equal(t, uint64(types.ReceiptSuccess), receipt.Status)
	}

	time.Sleep(2 * time.Second)
	require.Equal(t, sub.len(), 20)
}