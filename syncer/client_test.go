package syncer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xPolygon/polygon-edge/blockchain"
	"github.com/0xPolygon/polygon-edge/helper/tests"
	"github.com/0xPolygon/polygon-edge/network"
	"github.com/0xPolygon/polygon-edge/network/event"
	"github.com/0xPolygon/polygon-edge/network/grpc"
	"github.com/0xPolygon/polygon-edge/syncer/proto"
	"github.com/0xPolygon/polygon-edge/types"
)

var (
	networkConfig = func(c *network.Config) {
		c.NoDiscover = true
	}
)

func newTestNetwork(t *testing.T) *network.Server {
	t.Helper()

	srv, err := network.CreateServer(&network.CreateServerParams{
		ConfigCallback: networkConfig,
	})

	assert.NoError(t, err)

	return srv
}

func newTestSyncPeerClient(network Network, blockchain Blockchain) *syncPeerClient {
	client := &syncPeerClient{
		logger:                 hclog.NewNullLogger(),
		network:                network,
		blockchain:             blockchain,
		id:                     network.AddrInfo().ID.String(),
		peerStatusUpdateCh:     make(chan *NoForkPeer, 1),
		peerConnectionUpdateCh: make(chan *event.PeerEvent, 1),
		closeCh:                make(chan struct{}),
	}

	// need to register protocol
	network.RegisterProtocol(syncerProto, grpc.NewGrpcStream())

	return client
}

func createTestSyncerService(t *testing.T, chain Blockchain) (*syncPeerService, *network.Server) {
	t.Helper()

	srv := newTestNetwork(t)

	service := &syncPeerService{
		blockchain: chain,
		network:    srv,
	}

	service.Start()

	return service, srv
}

func TestGetPeerStatus(t *testing.T) {
	t.Parallel()

	clientSrv := newTestNetwork(t)
	client := newTestSyncPeerClient(clientSrv, nil)

	peerLatest := uint64(10)
	_, peerSrv := createTestSyncerService(t, &mockBlockchain{
		headerHandler: newSimpleHeaderHandler(peerLatest),
	})

	err := network.JoinAndWait(
		clientSrv,
		peerSrv,
		network.DefaultBufferTimeout,
		network.DefaultJoinTimeout,
	)

	assert.NoError(t, err)

	status, err := client.GetPeerStatus(peerSrv.AddrInfo().ID)
	assert.NoError(t, err)

	expected := &NoForkPeer{
		ID:       peerSrv.AddrInfo().ID,
		Number:   peerLatest,
		Distance: clientSrv.GetPeerDistance(peerSrv.AddrInfo().ID),
	}

	assert.Equal(t, expected, status)
}

func TestGetConnectedPeerStatuses(t *testing.T) {
	t.Parallel()

	clientSrv := newTestNetwork(t)
	client := newTestSyncPeerClient(clientSrv, nil)

	var (
		peerLatests = []uint64{
			30,
			20,
			10,
		}

		mutex        = sync.Mutex{}
		peerJoinErrs = make([]error, len(peerLatests))
		expected     = make([]*NoForkPeer, len(peerLatests))

		wg sync.WaitGroup
	)

	for idx, latest := range peerLatests {
		idx, latest := idx, latest

		_, peerSrv := createTestSyncerService(t, &mockBlockchain{
			headerHandler: newSimpleHeaderHandler(latest),
		})

		peerID := peerSrv.AddrInfo().ID

		wg.Add(1)

		go func() {
			defer wg.Done()

			mutex.Lock()
			defer mutex.Unlock()

			peerJoinErrs[idx] = network.JoinAndWait(
				clientSrv,
				peerSrv,
				network.DefaultBufferTimeout,
				network.DefaultJoinTimeout,
			)

			expected[idx] = &NoForkPeer{
				ID:       peerID,
				Number:   latest,
				Distance: clientSrv.GetPeerDistance(peerID),
			}
		}()
	}

	wg.Wait()

	for _, err := range peerJoinErrs {
		assert.NoError(t, err)
	}

	statuses := client.GetConnectedPeerStatuses()

	// no need to check order
	assert.Equal(t, expected, sortNoForkPeers(statuses))
}

func TestStatusPubSub(t *testing.T) {
	t.Parallel()

	clientSrv := newTestNetwork(t)
	client := newTestSyncPeerClient(clientSrv, nil)

	_, peerSrv := createTestSyncerService(t, &mockBlockchain{})
	peerID := peerSrv.AddrInfo().ID

	go client.startPeerEventProcess()

	var (
		events []*event.PeerEvent
		mutex  sync.Mutex
		wg     sync.WaitGroup
	)

	wg.Add(1)

	go func() {
		defer wg.Done()

		for event := range client.GetPeerConnectionUpdateEventCh() {
			mutex.Lock()
			events = append(events, event)
			mutex.Unlock()
		}
	}()

	// Use TestTimeout to manage the overall test timeout
	tests.TestTimeout(t, 10*time.Second, func(ctx context.Context) {
		// Connect
		err := network.JoinAndWait(
			clientSrv,
			peerSrv,
			network.DefaultBufferTimeout,
			network.DefaultJoinTimeout,
		)
		require.NoError(t, err)

		// Disconnect
		err = network.DisconnectAndWait(
			clientSrv,
			peerID,
			network.DefaultLeaveTimeout,
		)
		require.NoError(t, err)

		// Wait for both events to be processed
		err = tests.WaitFor(ctx, func() bool {
			mutex.Lock()
			defer mutex.Unlock()

			return len(events) == 2
		})
		require.NoError(t, err)

		// Close channel and wait for events
		close(client.closeCh)
		wg.Wait()

		expected := []*event.PeerEvent{
			{
				PeerID: peerID,
				Type:   event.PeerConnected,
			},
			{
				PeerID: peerID,
				Type:   event.PeerDisconnected,
			},
		}

		assert.Equal(t, expected, events)
	})
}

func TestPeerConnectionUpdateEventCh(t *testing.T) {
	t.Parallel()

	clientSrv := newTestNetwork(t)
	client := newTestSyncPeerClient(clientSrv, nil)

	_, peerSrv := createTestSyncerService(t, &mockBlockchain{})
	peerID := peerSrv.AddrInfo().ID

	go client.startPeerEventProcess()

	var (
		events []*event.PeerEvent
		mutex  sync.Mutex
		wg     sync.WaitGroup
	)

	wg.Add(1)

	go func() {
		defer wg.Done()

		for event := range client.GetPeerConnectionUpdateEventCh() {
			mutex.Lock()
			events = append(events, event)
			mutex.Unlock()
		}
	}()

	// Use TestTimeout to manage the overall test timeout
	tests.TestTimeout(t, 10*time.Second, func(ctx context.Context) {
		// Connect
		err := network.JoinAndWait(
			clientSrv,
			peerSrv,
			network.DefaultBufferTimeout,
			network.DefaultJoinTimeout,
		)
		require.NoError(t, err)

		// Disconnect
		err = network.DisconnectAndWait(
			clientSrv,
			peerID,
			network.DefaultLeaveTimeout,
		)
		require.NoError(t, err)

		// Close channel and wait for events
		close(client.closeCh)
		wg.Wait()

		expected := []*event.PeerEvent{
			{
				PeerID: peerID,
				Type:   event.PeerConnected,
			},
			{
				PeerID: peerID,
				Type:   event.PeerDisconnected,
			},
		}

		assert.Equal(t, expected, events)
	})
}

// Make sure the peer shouldn't emit status if the shouldEmitBlocks flag is set.
// The subtests cannot contain t.Parallel() due to how
// the test is organized
//
//nolint:tparallel
func Test_shouldEmitBlocks(t *testing.T) {
	t.Parallel()

	var (
		// network layer
		clientSrv = newTestNetwork(t)
		peerSrv   = newTestNetwork(t)

		clientLatest = uint64(10)

		subscription = blockchain.NewMockSubscription()

		client = newTestSyncPeerClient(clientSrv, &mockBlockchain{
			subscription:  subscription,
			headerHandler: newSimpleHeaderHandler(clientLatest),
		})
	)

	t.Cleanup(func() {
		clientSrv.Close()
		peerSrv.Close()
		client.Close()
	})

	err := network.JoinAndWaitMultiple(
		network.DefaultJoinTimeout,
		clientSrv,
		peerSrv,
	)

	assert.NoError(t, err)

	// start gossip
	assert.NoError(t, client.startGossip())

	// start to subscribe blockchain events
	go client.startNewBlockProcess()

	// push latest block number to blockchain subscription
	pushSubscription := func(sub *blockchain.MockSubscription, latest uint64) {
		sub.Push(&blockchain.Event{
			NewChain: []*types.Header{
				{
					Number: latest,
				},
			},
		})
	}

	waitForContext := func(ctx context.Context) bool {
		select {
		case <-ctx.Done():
			return true
		case <-time.After(5 * time.Second):
			return false
		}
	}

	// create topic & subscribe in peer
	topic, err := peerSrv.NewTopic(statusTopicName, &proto.SyncPeerStatus{})
	assert.NoError(t, err)

	testGossip := func(t *testing.T, shouldEmit bool) {
		t.Helper()

		// context to be canceled when receiving status
		receiveContext, cancelContext := context.WithCancel(context.Background())
		defer cancelContext()

		assert.NoError(t, topic.Subscribe(func(_ interface{}, id peer.ID) {
			cancelContext()
		}))

		// need to wait for a few seconds to propagate subscribing
		time.Sleep(2 * time.Second)

		if shouldEmit {
			client.EnablePublishingPeerStatus()
		} else {
			client.DisablePublishingPeerStatus()
		}

		pushSubscription(subscription, clientLatest)

		canceled := waitForContext(receiveContext)

		assert.Equal(t, shouldEmit, canceled)
	}

	t.Run("should send own status via gossip if shouldEmitBlocks is set", func(t *testing.T) {
		testGossip(t, true)
	})

	t.Run("shouldn't send own status via gossip if shouldEmitBlocks is reset", func(t *testing.T) {
		testGossip(t, false)
	})
}

func Test_syncPeerClient_GetBlocks(t *testing.T) {
	t.Parallel()

	clientSrv := newTestNetwork(t)
	client := newTestSyncPeerClient(clientSrv, nil)

	var (
		peerLatest = uint64(10)
		syncFrom   = uint64(1)
	)

	_, peerSrv := createTestSyncerService(t, &mockBlockchain{
		headerHandler: newSimpleHeaderHandler(peerLatest),
		getBlockByNumberHandler: func(u uint64, b bool) (*types.Block, bool) {
			if u <= 10 {
				return &types.Block{
					Header: &types.Header{
						Number: u,
					},
				}, true
			}

			return nil, false
		},
	})

	err := network.JoinAndWait(
		clientSrv,
		peerSrv,
		network.DefaultBufferTimeout,
		network.DefaultJoinTimeout,
	)

	assert.NoError(t, err)

	blockStream, err := client.GetBlocks(peerSrv.AddrInfo().ID, syncFrom, 5*time.Second)
	assert.NoError(t, err)

	blocks := make([]*types.Block, 0, peerLatest)
	for block := range blockStream {
		blocks = append(blocks, block)
	}

	// hash is calculated on unmarshaling
	expected := createMockBlocks(10)
	for _, b := range expected {
		b.Header.ComputeHash()
	}

	assert.Equal(t, expected, blocks)
}

func Test_EmitMultipleBlocks(t *testing.T) {
	t.Parallel()

	var (
		// network layer
		clientSrv = newTestNetwork(t)
		peerSrv   = newTestNetwork(t)

		clientLatest = uint64(10)
		subscription = blockchain.NewMockSubscription()

		client = newTestSyncPeerClient(clientSrv, &mockBlockchain{
			subscription:  subscription,
			headerHandler: newSimpleHeaderHandler(clientLatest),
		})

		// add synchronization primitives
		wg   sync.WaitGroup
		mu   sync.Mutex
		done = make(chan struct{})
	)

	t.Cleanup(func() {
		close(done) // signal goroutines to stop
		wg.Wait()   // wait for goroutines to finish

		clientSrv.Close()
		peerSrv.Close()
		client.Close()
	})

	err := network.JoinAndWaitMultiple(
		network.DefaultJoinTimeout,
		clientSrv,
		peerSrv,
	)
	require.NoError(t, err)

	// start gossip
	require.NoError(t, client.startGossip())

	// start to subscribe blockchain events with proper cleanup
	wg.Add(1)

	go func() {
		defer wg.Done()
		client.startNewBlockProcess()
	}()

	// push latest block number to blockchain subscription
	pushSubscription := func(sub *blockchain.MockSubscription, latest uint64) {
		sub.Push(&blockchain.Event{
			NewChain: []*types.Header{
				{
					Number: latest,
				},
			},
		})
	}

	waitForGossip := func(wg *sync.WaitGroup) bool {
		c := make(chan struct{})

		go func() {
			defer close(c)
			wg.Wait()
		}()

		select {
		case <-c:
			return true
		case <-time.After(5 * time.Second):
			return false
		}
	}

	// create topic & subscribe in peer
	topic, err := peerSrv.NewTopic(statusTopicName, &proto.SyncPeerStatus{})
	require.NoError(t, err)

	testGossip := func(t *testing.T, blocksNum int) {
		t.Helper()

		var (
			wgForGossip sync.WaitGroup
			messages    []*proto.SyncPeerStatus
		)

		wgForGossip.Add(blocksNum)

		// subscribe and collect messages
		require.NoError(t, topic.Subscribe(func(msg interface{}, _ peer.ID) {
			mu.Lock()
			if status, ok := msg.(*proto.SyncPeerStatus); ok {
				messages = append(messages, status)
			}
			mu.Unlock()
			wgForGossip.Done()
		}))

		// need to wait for a few seconds to propagate subscribing
		time.Sleep(2 * time.Second)
		client.EnablePublishingPeerStatus()

		// send blocks in sequence
		for i := 0; i < blocksNum; i++ {
			pushSubscription(subscription, clientLatest+uint64(i))
		}

		// wait for all messages to be received
		gossiped := waitForGossip(&wgForGossip)
		require.True(t, gossiped, "Failed to receive all gossip messages")

		// verify messages
		mu.Lock()
		defer mu.Unlock()

		require.Len(t, messages, blocksNum, "Should receive exactly %d messages", blocksNum)

		// verify message contents are in sequence
		for i, msg := range messages {
			expectedNumber := clientLatest + uint64(i)
			assert.Equal(t, expectedNumber, msg.Number,
				"Message %d should have number %d", i, expectedNumber)
		}
	}

	t.Run("should receive all blocks", func(t *testing.T) {
		t.Parallel()
		testGossip(t, 4)
	})
}
