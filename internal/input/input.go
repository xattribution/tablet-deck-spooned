// Package input presses keys on this PC and tells whether the game is the window in
// front. On Windows it calls SendInput directly; elsewhere (and in tests) a fake driver
// stands in, so everything above it can run anywhere.
package input

import (
	"strings"
	"sync"
	"time"

	"github.com/xattribution/tablet-deck-spooned/internal/keys"
)

// Game is what the driver knows about the game right now.
type Game struct {
	Running bool   `json:"running"`
	Focused bool   `json:"focused"` // the game owns the window in front
	PID     uint32 `json:"pid,omitempty"`
	Exe     string `json:"exe,omitempty"`   // full path, for finding its keybinds
	Front   string `json:"front,omitempty"` // process name of the window in front
}

// Driver presses keys and watches the game window.
type Driver interface {
	// Key sends one key down or up. Dry-run drivers only record it.
	Key(sc keys.Scan, down bool) error
	// Game reports the game's state; cheap enough to call before every press.
	Game() Game
	// BringToFront makes the game the window in front, waiting up to timeout.
	BringToFront(timeout time.Duration) bool
	// SetGames sets the process names counted as the game ("StarCitizen.exe").
	SetGames(names []string)
	Name() string
}

// NormalizeGames lowercases process names and adds ".exe" where it's missing.
func NormalizeGames(names []string) []string {
	var out []string
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if n == "" {
			continue
		}
		if !strings.HasSuffix(n, ".exe") {
			n += ".exe"
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		out = []string{"starcitizen.exe"}
	}
	return out
}

// KeyEvent is one key the fake driver was asked to send.
type KeyEvent struct {
	Scan keys.Scan
	Down bool
	At   time.Time
}

// Fake is a driver for tests and non-Windows dry runs. Its game state is set by hand.
type Fake struct {
	mu        sync.Mutex
	game      Game
	events    []KeyEvent
	refocusOK bool
	OnKey     func(KeyEvent)
}

// NewFake returns a fake driver with the game running and in front.
func NewFake() *Fake {
	return &Fake{game: Game{Running: true, Focused: true, Front: "starcitizen.exe"}, refocusOK: true}
}

func (f *Fake) Name() string { return "fake" }

func (f *Fake) Key(sc keys.Scan, down bool) error {
	f.mu.Lock()
	ev := KeyEvent{Scan: sc, Down: down, At: time.Now()}
	f.events = append(f.events, ev)
	cb := f.OnKey
	f.mu.Unlock()
	if cb != nil {
		cb(ev)
	}
	return nil
}

func (f *Fake) Game() Game {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.game
}

func (f *Fake) BringToFront(time.Duration) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.game.Running || !f.refocusOK {
		return false
	}
	f.game.Focused = true
	return true
}

func (f *Fake) SetGames([]string) {}

// SetGame changes what the fake reports; refocusOK is whether BringToFront works.
func (f *Fake) SetGame(running, focused, refocusOK bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.game.Running, f.game.Focused, f.refocusOK = running, focused, refocusOK
}

// Events returns and clears the recorded keys.
func (f *Fake) Events() []KeyEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.events
	f.events = nil
	return out
}
