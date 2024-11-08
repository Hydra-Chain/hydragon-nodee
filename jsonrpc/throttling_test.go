package jsonrpc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThrottling(t *testing.T) {
	t.Parallel()

	th := NewThrottling(5, time.Millisecond*50)
	sfn := func(value int, sleep time.Duration) func() (interface{}, error) {
		return func() (interface{}, error) {
			time.Sleep(sleep)

			return value, nil
		}
	}

	var wg sync.WaitGroup

	startCh := make(chan struct{})

	// Function to attempt a request with context
	attemptRequest := func(ctx context.Context, value int, sleep time.Duration, expectError bool) {
		defer wg.Done()
		<-startCh

		res, err := th.AttemptRequest(ctx, sfn(value, sleep))
		if expectError {
			require.ErrorIs(t, err, errRequestLimitExceeded)
			assert.Nil(t, res)
		} else {
			require.NoError(t, err)

			if intValue, ok := res.(int); ok {
				assert.Equal(t, value, intValue)
			} else {
				assert.Fail(t, "type assertion failed")
			}
		}
	}

	wg.Add(9)

	// Start goroutines
	go attemptRequest(context.Background(), 100, time.Millisecond*500, false)
	time.Sleep(time.Millisecond * 100)

	for i := 2; i <= 5; i++ {
		go attemptRequest(context.Background(), 100, time.Millisecond*1000, false)
	}

	go func() {
		time.Sleep(time.Millisecond * 150)
		attemptRequest(context.Background(), 100, time.Millisecond*100, true)
	}()

	go func() {
		time.Sleep(time.Millisecond * 620)
		attemptRequest(context.Background(), 10, time.Millisecond*100, false)
	}()

	go func() {
		time.Sleep(time.Millisecond * 640)
		attemptRequest(context.Background(), 100, time.Millisecond*100, true)
	}()

	go func() {
		time.Sleep(time.Millisecond * 1000)
		attemptRequest(context.Background(), 1, time.Millisecond*100, false)
	}()

	// Start all requests
	close(startCh)
	wg.Wait()
}
