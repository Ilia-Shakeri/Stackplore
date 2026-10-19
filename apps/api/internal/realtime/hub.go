// Package realtime provides bounded transient fan-out for browser streams.
package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrClosed         = errors.New("realtime feed is closed")
	ErrInvalidFrame   = errors.New("realtime frame is invalid")
	ErrTooManyFeeds   = errors.New("realtime feed limit reached")
	ErrInvalidOptions = errors.New("realtime options are invalid")
	ErrRevoked        = errors.New("realtime feed authority was revoked")
)

type Frame struct {
	ID        uint64
	Event     string
	Data      []byte
	CreatedAt time.Time
}

type Producer func(context.Context, func(event string, data []byte) error) error

type ManagerOptions struct {
	MaximumFeeds int
	ReplaySize   int
	QueueSize    int
	MaximumFrame int
	Now          func() time.Time
}

type Manager struct {
	mu      sync.Mutex
	options ManagerOptions
	feeds   map[string]*managedFeed
	closed  bool
}

type managedFeed struct {
	manager     *Manager
	key         string
	hub         *hub
	cancel      context.CancelFunc
	done        chan struct{}
	subscribers int
	producer    Producer
}

type hub struct {
	mu           sync.Mutex
	replay       []Frame
	replaySize   int
	queueSize    int
	maximumFrame int
	nextID       uint64
	nextSubID    uint64
	subscribers  map[uint64]chan Frame
	closed       bool
	overflows    atomic.Uint64
	revoked      atomic.Bool
	now          func() time.Time
}

type Subscription struct {
	C         <-chan Frame
	Reset     bool
	closeOnce sync.Once
	close     func()
	revoked   *atomic.Bool
}

func NewManager(options ManagerOptions) (*Manager, error) {
	if options.MaximumFeeds < 1 || options.MaximumFeeds > 1024 || options.ReplaySize < 1 || options.ReplaySize > 4096 ||
		options.QueueSize < 1 || options.QueueSize > options.ReplaySize || options.MaximumFrame < 256 || options.MaximumFrame > 1024*1024 {
		return nil, ErrInvalidOptions
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Manager{options: options, feeds: make(map[string]*managedFeed)}, nil
}

func (manager *Manager) Subscribe(key string, lastID uint64, producer Producer) (*Subscription, error) {
	if manager == nil || key == "" || len(key) > 256 || producer == nil {
		return nil, ErrInvalidOptions
	}
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return nil, ErrClosed
	}
	feed := manager.feeds[key]
	newFeed := false
	var producerContext context.Context
	if feed == nil {
		if len(manager.feeds) >= manager.options.MaximumFeeds {
			manager.mu.Unlock()
			return nil, ErrTooManyFeeds
		}
		ctx, cancel := context.WithCancel(context.Background())
		producerContext = ctx
		feed = &managedFeed{
			manager: manager, key: key, cancel: cancel, done: make(chan struct{}), producer: producer,
			hub: newHub(manager.options),
		}
		manager.feeds[key] = feed
		newFeed = true
	}
	feed.subscribers++
	manager.mu.Unlock()

	channel, reset, closeHub, err := feed.hub.subscribe(lastID)
	if err != nil {
		feed.release()
		return nil, err
	}
	if newFeed {
		go feed.run(producerContext)
	}
	subscription := &Subscription{C: channel, Reset: reset, revoked: &feed.hub.revoked}
	subscription.close = func() {
		closeHub()
		feed.release()
	}
	return subscription, nil
}

func (subscription *Subscription) Close() {
	if subscription != nil {
		subscription.closeOnce.Do(subscription.close)
	}
}

func (subscription *Subscription) Revoked() bool {
	return subscription != nil && subscription.revoked != nil && subscription.revoked.Load()
}

// Invalidate discards a scope's history and queued frames before closing its
// subscriptions. A replacement feed never shares the revoked producer or ring.
func (manager *Manager) Invalidate(key string) {
	if manager == nil {
		return
	}
	manager.mu.Lock()
	if feed := manager.feeds[key]; feed != nil {
		feed.invalidateLocked()
	}
	manager.mu.Unlock()
}

func (manager *Manager) Close() {
	if manager == nil {
		return
	}
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return
	}
	manager.closed = true
	feeds := make([]*managedFeed, 0, len(manager.feeds))
	for _, feed := range manager.feeds {
		feeds = append(feeds, feed)
		feed.cancel()
	}
	manager.feeds = make(map[string]*managedFeed)
	manager.mu.Unlock()
	for _, feed := range feeds {
		<-feed.done
	}
}

func (manager *Manager) ActiveFeeds() int {
	if manager == nil {
		return 0
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return len(manager.feeds)
}

func (feed *managedFeed) run(ctx context.Context) {
	defer close(feed.done)
	if errors.Is(feed.producer(ctx, feed.hub.publish), ErrRevoked) {
		feed.manager.mu.Lock()
		feed.invalidateLocked()
		feed.manager.mu.Unlock()
		return
	}
	feed.hub.close()
}

func (feed *managedFeed) invalidateLocked() {
	if feed.manager.feeds[feed.key] == feed {
		delete(feed.manager.feeds, feed.key)
	}
	feed.hub.mu.Lock()
	feed.hub.revoked.Store(true)
	feed.hub.replay = nil
	if !feed.hub.closed {
		feed.hub.closed = true
		for id, subscriber := range feed.hub.subscribers {
		drain:
			for {
				select {
				case <-subscriber:
				default:
					break drain
				}
			}
			close(subscriber)
			delete(feed.hub.subscribers, id)
		}
	}
	feed.hub.mu.Unlock()
	// A canceled producer may close the hub immediately, so commit revocation
	// and discard queued data before allowing its cancellation path to run.
	feed.cancel()
}

func (feed *managedFeed) release() {
	feed.manager.mu.Lock()
	if feed.subscribers > 0 {
		feed.subscribers--
	}
	if feed.subscribers == 0 && feed.manager.feeds[feed.key] == feed {
		delete(feed.manager.feeds, feed.key)
		feed.cancel()
	}
	feed.manager.mu.Unlock()
}

func newHub(options ManagerOptions) *hub {
	return &hub{
		replaySize: options.ReplaySize, queueSize: options.QueueSize, maximumFrame: options.MaximumFrame,
		subscribers: make(map[uint64]chan Frame), now: options.Now,
	}
}

func (hub *hub) publish(event string, data []byte) error {
	if !validEventName(event) || len(data) == 0 || len(data) > hub.maximumFrame || !json.Valid(data) {
		return ErrInvalidFrame
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if hub.closed {
		return ErrClosed
	}
	hub.nextID++
	frame := Frame{ID: hub.nextID, Event: event, Data: slices.Clone(data), CreatedAt: hub.now().UTC().Truncate(time.Microsecond)}
	hub.replay = append(hub.replay, frame)
	if len(hub.replay) > hub.replaySize {
		hub.replay = slices.Clone(hub.replay[len(hub.replay)-hub.replaySize:])
	}
	for id, subscriber := range hub.subscribers {
		select {
		case subscriber <- cloneFrame(frame):
		default:
			close(subscriber)
			delete(hub.subscribers, id)
			hub.overflows.Add(1)
		}
	}
	return nil
}

func (hub *hub) subscribe(lastID uint64) (<-chan Frame, bool, func(), error) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if hub.closed {
		return nil, false, nil, ErrClosed
	}
	reset := false
	replay := []Frame(nil)
	if lastID > 0 {
		if len(hub.replay) == 0 || lastID > hub.nextID || lastID+1 < hub.replay[0].ID {
			reset = true
		} else {
			for _, frame := range hub.replay {
				if frame.ID > lastID {
					replay = append(replay, cloneFrame(frame))
				}
			}
		}
	}
	capacity := max(hub.queueSize, len(replay))
	channel := make(chan Frame, capacity)
	for _, frame := range replay {
		channel <- frame
	}
	hub.nextSubID++
	id := hub.nextSubID
	hub.subscribers[id] = channel
	var once sync.Once
	closeSubscription := func() {
		once.Do(func() {
			hub.mu.Lock()
			if current, exists := hub.subscribers[id]; exists {
				delete(hub.subscribers, id)
				close(current)
			}
			hub.mu.Unlock()
		})
	}
	return channel, reset, closeSubscription, nil
}

func (hub *hub) close() {
	hub.mu.Lock()
	if !hub.closed {
		hub.closed = true
		for id, subscriber := range hub.subscribers {
			close(subscriber)
			delete(hub.subscribers, id)
		}
	}
	hub.mu.Unlock()
}

func cloneFrame(frame Frame) Frame {
	frame.Data = slices.Clone(frame.Data)
	return frame
}

func validEventName(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	for index, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || index > 0 && (character == '.' || character == '-' || character == '_') {
			continue
		}
		return false
	}
	return true
}
