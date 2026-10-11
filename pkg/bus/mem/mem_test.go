package mem_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qamati/IOT_V2/pkg/bus"
	"github.com/qamati/IOT_V2/pkg/bus/mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPubSubBasic(t *testing.T) {
	b := mem.New(4)
	defer b.Close()

	ctx := context.Background()
	received := make(chan *bus.Message, 1)

	sub, err := b.Subscribe(ctx, "telemetry.up", "", func(ctx context.Context, msg *bus.Message) error {
		received <- msg
		return nil
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	msg := bus.NewMessage("tenant-1", "DEVICE", "dev-100", bus.TypeTelemetryUp, []byte(`{"temp":24.5}`))
	err = b.Publish(ctx, "telemetry.up", msg)
	require.NoError(t, err)

	select {
	case got := <-received:
		assert.Equal(t, msg.ID, got.ID)
		assert.Equal(t, "tenant-1", got.TenantID)
		assert.Equal(t, "dev-100", got.OriginatorID)
		assert.Equal(t, `{"temp":24.5}`, string(got.Payload))
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func TestPerKeyOrdering(t *testing.T) {
	b := mem.New(4)
	defer b.Close()

	ctx := context.Background()
	const count = 50
	var receivedSeq []int
	var mu sync.Mutex
	done := make(chan struct{})

	_, err := b.Subscribe(ctx, "ordered.topic", "", func(ctx context.Context, msg *bus.Message) error {
		mu.Lock()
		defer mu.Unlock()
		var seq int
		_, _ = fmt.Sscanf(string(msg.Payload), "%d", &seq)
		receivedSeq = append(receivedSeq, seq)
		if len(receivedSeq) == count {
			close(done)
		}
		return nil
	})
	require.NoError(t, err)

	// Publish count messages for the SAME originator ID
	for i := 0; i < count; i++ {
		msg := bus.NewMessage("tenant-1", "DEVICE", "same-device-1", "EVENT", []byte(fmt.Sprintf("%d", i)))
		require.NoError(t, b.Publish(ctx, "ordered.topic", msg))
	}

	select {
	case <-done:
		mu.Lock()
		defer mu.Unlock()
		for i := 0; i < count; i++ {
			assert.Equal(t, i, receivedSeq[i], "Message order must be strictly preserved per originator key")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for ordered messages")
	}
}

func TestConsumerGroups(t *testing.T) {
	b := mem.New(4)
	defer b.Close()

	ctx := context.Background()
	var countSub1 int32
	var countSub2 int32
	const totalMessages = 40
	var wg sync.WaitGroup
	wg.Add(totalMessages)

	groupName := "workers"
	_, err := b.Subscribe(ctx, "job.queue", groupName, func(ctx context.Context, msg *bus.Message) error {
		atomic.AddInt32(&countSub1, 1)
		wg.Done()
		return nil
	})
	require.NoError(t, err)

	_, err = b.Subscribe(ctx, "job.queue", groupName, func(ctx context.Context, msg *bus.Message) error {
		atomic.AddInt32(&countSub2, 1)
		wg.Done()
		return nil
	})
	require.NoError(t, err)

	// Publish messages across different device IDs
	for i := 0; i < totalMessages; i++ {
		msg := bus.NewMessage("tenant-1", "DEVICE", fmt.Sprintf("dev-%d", i), "JOB", []byte("data"))
		require.NoError(t, b.Publish(ctx, "job.queue", msg))
	}

	wg.Wait()

	total := atomic.LoadInt32(&countSub1) + atomic.LoadInt32(&countSub2)
	assert.Equal(t, int32(totalMessages), total)
	assert.True(t, atomic.LoadInt32(&countSub1) > 0, "Subscriber 1 should receive messages")
	assert.True(t, atomic.LoadInt32(&countSub2) > 0, "Subscriber 2 should receive messages")
}
