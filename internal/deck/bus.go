package deck

import (
	"sync"
	"time"
)

// Event is something that happened, for decks, the control panel and integrations.
type Event struct {
	T    string    `json:"t"`
	Time time.Time `json:"time"`
	Data any       `json:"data,omitempty"`
}

// Bus fans events out to subscribers. A subscriber that falls behind loses events
// rather than slowing presses down.
type Bus struct {
	mu      sync.Mutex
	subs    map[chan Event]struct{}
	history []Event
	keep    int
}

func NewBus(keep int) *Bus {
	return &Bus{subs: map[chan Event]struct{}{}, keep: keep}
}

// Publish sends an event to every subscriber and keeps it in the recent history.
func (b *Bus) Publish(t string, data any) {
	ev := Event{T: t, Time: time.Now(), Data: data}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.keep > 0 {
		b.history = append(b.history, ev)
		if len(b.history) > b.keep {
			b.history = b.history[len(b.history)-b.keep:]
		}
	}
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Subscribe returns a channel of events and a function that ends the subscription.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 128)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs, ch)
			b.mu.Unlock()
		})
	}
}

// History returns recent events, oldest first.
func (b *Bus) History() []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]Event(nil), b.history...)
}
