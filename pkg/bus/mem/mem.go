package mem

import (
	"context"
	"errors"
	"hash/fnv"
	"sync"

	"github.com/qamati/IOT_V2/pkg/bus"
)

const defaultPartitions = 8
const defaultQueueSize = 256

type subscriber struct {
	id      int
	topic   string
	group   string
	handler bus.Handler
}

// MemoryBus implements an in-memory bus.Bus with partitioned per-key ordering.
type MemoryBus struct {
	mu          sync.RWMutex
	subscribers map[string][]*subscriber // topic -> list of subscribers
	nextSubID   int
	closed      bool

	partitions int
	chans      []chan task
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

type task struct {
	ctx     context.Context
	msg     *bus.Message
	handler bus.Handler
}

// New creates a new in-memory message bus.
func New(partitions int) *MemoryBus {
	if partitions <= 0 {
		partitions = defaultPartitions
	}

	ctx, cancel := context.WithCancel(context.Background())
	b := &MemoryBus{
		subscribers: make(map[string][]*subscriber),
		partitions:  partitions,
		chans:       make([]chan task, partitions),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start partition workers
	for i := 0; i < partitions; i++ {
		ch := make(chan task, defaultQueueSize)
		b.chans[i] = ch
		b.wg.Add(1)
		go b.worker(ch)
	}

	return b
}

func (b *MemoryBus) worker(ch <-chan task) {
	defer b.wg.Done()
	for {
		select {
		case <-b.ctx.Done():
			// Drain remaining tasks
			for t := range ch {
				_ = t.handler(t.ctx, t.msg)
			}
			return
		case t, ok := <-ch:
			if !ok {
				return
			}
			_ = t.handler(t.ctx, t.msg)
		}
	}
}

func (b *MemoryBus) partitionIndex(key string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32()) % b.partitions
}

// Publish dispatches a message to subscribers matching the topic.
func (b *MemoryBus) Publish(ctx context.Context, topic string, msg *bus.Message) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return errors.New("mem bus: publish on closed bus")
	}

	subs, ok := b.subscribers[topic]
	if !ok || len(subs) == 0 {
		return nil
	}

	// Group subscribers by group name
	groups := make(map[string][]*subscriber)
	var ungrouped []*subscriber

	for _, sub := range subs {
		if sub.group == "" {
			ungrouped = append(ungrouped, sub)
		} else {
			groups[sub.group] = append(groups[sub.group], sub)
		}
	}

	partIdx := b.partitionIndex(msg.PartitionKey())

	// Dispatch to ungrouped (broadcast)
	for _, sub := range ungrouped {
		b.chans[partIdx] <- task{
			ctx:     ctx,
			msg:     msg,
			handler: sub.handler,
		}
	}

	// Dispatch to grouped (partition-consistent round-robin / modulo per group)
	for _, groupSubs := range groups {
		chosenIdx := partIdx % len(groupSubs)
		chosenSub := groupSubs[chosenIdx]
		b.chans[partIdx] <- task{
			ctx:     ctx,
			msg:     msg,
			handler: chosenSub.handler,
		}
	}

	return nil
}

type memSubscription struct {
	bus   *MemoryBus
	topic string
	subID int
}

func (s *memSubscription) Unsubscribe() error {
	s.bus.mu.Lock()
	defer s.bus.mu.Unlock()

	subs := s.bus.subscribers[s.topic]
	for i, sub := range subs {
		if sub.id == s.subID {
			s.bus.subscribers[s.topic] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	return nil
}

// Subscribe registers a message handler on a topic.
func (b *MemoryBus) Subscribe(ctx context.Context, topic string, group string, handler bus.Handler) (bus.Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, errors.New("mem bus: subscribe on closed bus")
	}

	b.nextSubID++
	sub := &subscriber{
		id:      b.nextSubID,
		topic:   topic,
		group:   group,
		handler: handler,
	}

	b.subscribers[topic] = append(b.subscribers[topic], sub)

	return &memSubscription{
		bus:   b,
		topic: topic,
		subID: sub.id,
	}, nil
}

// Close gracefully stops the bus workers.
func (b *MemoryBus) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()

	b.cancel()
	for _, ch := range b.chans {
		close(ch)
	}
	b.wg.Wait()
	return nil
}
