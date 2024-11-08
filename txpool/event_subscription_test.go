package txpool

import (
	"context"
	cryptoRand "crypto/rand"
	"math/big"
	mathRand "math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xPolygon/polygon-edge/helper/tests"
	"github.com/0xPolygon/polygon-edge/txpool/proto"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/assert"
)

func shuffleTxPoolEvents(
	supportedTypes []proto.EventType,
	count int,
	numInvalid int,
) []*proto.TxPoolEvent {
	if count == 0 || len(supportedTypes) == 0 {
		return []*proto.TxPoolEvent{}
	}

	if numInvalid > count {
		numInvalid = count
	}

	allEvents := []proto.EventType{
		proto.EventType_ADDED,
		proto.EventType_PROMOTED,
		proto.EventType_PROMOTED,
		proto.EventType_DROPPED,
		proto.EventType_DEMOTED,
	}
	txHash := types.StringToHash("123")

	tempSubscription := &eventSubscription{eventTypes: supportedTypes}

	randomEventType := func(supported bool) proto.EventType {
		for {
			randNum, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(len(supportedTypes))))

			randType := allEvents[randNum.Int64()]
			if tempSubscription.eventSupported(randType) == supported {
				return randType
			}
		}
	}

	events := make([]*proto.TxPoolEvent, 0)

	// Fill in the unsupported events first
	for invalidFilled := 0; invalidFilled < numInvalid; invalidFilled++ {
		events = append(events, &proto.TxPoolEvent{
			TxHash: txHash.String(),
			Type:   randomEventType(false),
		})
	}

	// Fill in the supported events
	for validFilled := 0; validFilled < count-numInvalid; validFilled++ {
		events = append(events, &proto.TxPoolEvent{
			TxHash: txHash.String(),
			Type:   randomEventType(true),
		})
	}

	// Shuffle the events
	mathRand.Seed(time.Now().UTC().UnixNano())
	mathRand.Shuffle(len(events), func(i, j int) {
		events[i], events[j] = events[j], events[i]
	})

	return events
}

func TestEventSubscription_ProcessedEvents(t *testing.T) {
	t.Parallel()

	// Set up the default values
	supportedEvents := []proto.EventType{
		proto.EventType_ADDED,
		proto.EventType_ENQUEUED,
		proto.EventType_DROPPED,
	}

	testTable := []struct {
		name              string
		events            []*proto.TxPoolEvent
		expectedProcessed int
	}{
		{
			"All supported events processed",
			shuffleTxPoolEvents(supportedEvents, 10, 0),
			10,
		},
		{
			"All unsupported events not processed",
			shuffleTxPoolEvents(supportedEvents, 10, 10),
			0,
		},
		{
			"Mixed events processed",
			shuffleTxPoolEvents(supportedEvents, 10, 6),
			4,
		},
	}

	for _, testCase := range testTable {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tests.TestTimeout(t, 10*time.Second, func(ctx context.Context) {
				subscription := &eventSubscription{
					eventTypes: supportedEvents,
					outputCh:   make(chan *proto.TxPoolEvent, len(tc.events)),
					doneCh:     make(chan struct{}),
					eventStore: &eventQueue{
						events: make([]*proto.TxPoolEvent, 0),
					},
					notifyCh: make(chan struct{}),
				}

				var wg sync.WaitGroup

				wg.Add(1)

				go func() {
					defer wg.Done()
					subscription.runLoop()
				}()

				processed := int64(0)
				processingDone := make(chan struct{})

				go func() {
					defer close(processingDone)

					for {
						select {
						case <-ctx.Done():
							return
						case event, ok := <-subscription.outputCh: // Set the event listener
							if !ok {
								return
							}

							if event != nil {
								atomic.AddInt64(&processed, 1)
							}
						}
					}
				}()

				// Push events and wait for each one
				for _, event := range tc.events {
					if err := tests.WaitFor(ctx, func() bool {
						subscription.pushEvent(event)

						return true
					}); err != nil {
						t.Fatal("failed to push event:", err)
					}
				}

				// Wait for all events to be processed
				if err := tests.WaitFor(ctx, func() bool {
					return atomic.LoadInt64(&processed) >= int64(tc.expectedProcessed)
				}); err != nil {
					t.Fatalf("timeout waiting for events to be processed. Expected %d, got %d",
						tc.expectedProcessed,
						atomic.LoadInt64(&processed))
				}

				// Cleanup
				cleanup := make(chan struct{})
				go func() {
					subscription.close()
					wg.Wait()
					close(cleanup)
				}()

				// Wait for cleanup
				if err := tests.WaitFor(ctx, func() bool {
					select {
					case <-cleanup:
						return true
					default:
						return false
					}
				}); err != nil {
					t.Fatal("cleanup timeout:", err)
				}

				// Final verification
				processedCount := atomic.LoadInt64(&processed)
				assert.Equal(t,
					int64(tc.expectedProcessed),
					processedCount,
					"Expected %d processed events, got %d",
					tc.expectedProcessed,
					processedCount,
				)
			})
		})
	}
}

func TestEventSubscription_EventSupported(t *testing.T) {
	t.Parallel()

	supportedEvents := []proto.EventType{
		proto.EventType_ADDED,
		proto.EventType_PROMOTED,
		proto.EventType_DEMOTED,
	}

	subscription := &eventSubscription{
		eventTypes: supportedEvents,
	}

	testTable := []struct {
		name      string
		events    []proto.EventType
		supported bool
	}{
		{
			"Supported events processed",
			supportedEvents,
			true,
		},
		{
			"Unsupported events not processed",
			[]proto.EventType{
				proto.EventType_DROPPED,
				proto.EventType_ENQUEUED,
			},
			false,
		},
	}

	for _, testCase := range testTable {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			for _, eventType := range testCase.events {
				assert.Equal(t, testCase.supported, subscription.eventSupported(eventType))
			}
		})
	}
}
