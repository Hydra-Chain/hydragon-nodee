package priceoracle

import (
	"testing"
	"time"

	"github.com/0xPolygon/polygon-edge/blockchain"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/require"
)

func TestBlockMustBeProcessed(t *testing.T) {
	// Mock structures
	mockBlockchainBackend := new(MockBlockchainBackend)
	priceOracle := &PriceOracle{
		blockchain: mockBlockchainBackend,
	}

	tests := []struct {
		name          string
		event         *blockchain.Event
		currentHeader *types.Header
		expected      bool
	}{
		{
			name: "block timestamp is older than 2 minutes",
			event: &blockchain.Event{
				NewChain: []*types.Header{
					{
						Timestamp: uint64(time.Now().Add(-3 * time.Minute).Unix()),
						Number:    5,
					},
				},
			},
			currentHeader: &types.Header{Number: 6},
			expected:      false,
		},
		{
			name: "block number is smaller than current header",
			event: &blockchain.Event{
				NewChain: []*types.Header{
					{
						Timestamp: uint64(time.Now().Unix()),
						Number:    5,
					},
				},
			},
			currentHeader: &types.Header{Number: 6},
			expected:      false,
		},
		{
			name: "event type is fork",
			event: &blockchain.Event{
				NewChain: []*types.Header{
					{
						Timestamp: uint64(time.Now().Unix()),
						Number:    7,
					},
				},
				Type: blockchain.EventFork,
			},
			currentHeader: &types.Header{Number: 6},
			expected:      false,
		},
		{
			name: "successful case",
			event: &blockchain.Event{
				NewChain: []*types.Header{
					{
						Timestamp: uint64(time.Now().Unix()),
						Number:    7,
					},
				},
				Type: blockchain.EventHead,
			},
			currentHeader: &types.Header{Number: 6},
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the expected behavior for the mock
			mockBlockchainBackend.On("CurrentHeader").Return(tt.currentHeader)

			// Call the method under test
			result := priceOracle.blockMustBeProcessed(tt.event)

			// Assert the result
			require.Equal(t, tt.expected, result)
		})
	}
}
