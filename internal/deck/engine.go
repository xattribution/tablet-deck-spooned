// Package deck turns button presses into keys. Every key goes through one worker
// goroutine, so combos never interleave, a key held by two buttons is only released
// when both let go, and a key that went down always comes back up.
package deck

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/xattribution/tablet-deck-spooned/internal/input"
	"github.com/xattribution/tablet-deck-spooned/internal/keys"
	"github.com/xattribution/tablet-deck-spooned/internal/layout"
)

// Focus modes: when may a press reach the keyboard?
const (
	FocusStrict = "strict" // only while the game is the window in front
	FocusLocal  = "local"  // presses from this PC bring the game to front first; others are strict
	FocusAlways = "always" // every press brings the game to front first
	FocusOff    = "off"    // keys go to whatever window is in front
)

// Reasons a press didn't go through, or a hold ended early.
const (
	ReasonNotRunning  = "not-running"
	ReasonNotFocused  = "not-focused"
	ReasonFocusFailed = "focus-failed"
	ReasonFocusLost   = "focus-lost"
	ReasonMaxHold     = "max-hold"
	ReasonReleaseAll  = "release-all"
	ReasonDenied      = "denied"
	ReasonRateLimited = "rate-limited"
)

// Settings are read on every press, so changes apply at once.
type Settings struct {
	TapMs              int    // how long a tap holds its keys
	ComboGapMs         int    // pause between the keys of a combo
	FocusMode          string // see the Focus* constants
	SettleMs           int    // after bringing the game to front, before the first key
	ReleaseOnFocusLoss bool   // let go of held keys when the game loses focus
	MaxHoldMs          int    // let go of a hold after this long; 0 = never
}

// DefaultSettings are what a fresh install uses.
func DefaultSettings() Settings {
	return Settings{TapMs: 60, ComboGapMs: 12, FocusMode: FocusLocal, SettleMs: 45, ReleaseOnFocusLoss: true, MaxHoldMs: 120000}
}

// Source says where a press came from.
type Source struct {
	Kind  string `json:"kind"`            // deck, voice, api, guest, peer
	Who   string `json:"who,omitempty"`   // device or guest name
	Local bool   `json:"local,omitempty"` // from this PC (its touchscreen, Stream Deck, voice)
}

// Result of a press.
type Result struct {
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
}

// Hold is a button being held down.
type Hold struct {
	ID       uint64
	Button   *layout.Button
	Src      Source
	Started  time.Time
	scans    []keys.Scan
	released bool
	lifted   []keys.Scan // other buttons' modifiers lifted while this one is down
	// OnRelease runs (on the worker) if the hold is ended by the engine, not the owner.
	OnRelease func(reason string)
}

// Engine presses keys.
type Engine struct {
	drv      input.Driver
	settings atomic.Pointer[Settings]
	Bus      *Bus

	jobs   chan func()
	down   map[keys.Scan]int // worker only
	holds  map[uint64]*Hold  // worker only
	nextID uint64
	active atomic.Int32 // holds in progress, for the focus watcher

	stMu   sync.Mutex
	states map[string]bool

	presses atomic.Int64
	blocked atomic.Int64
	stop    chan struct{}
}

// New starts an engine on a driver.
func New(drv input.Driver, s Settings, bus *Bus) *Engine {
	e := &Engine{drv: drv, Bus: bus, jobs: make(chan func(), 256), down: map[keys.Scan]int{},
		holds: map[uint64]*Hold{}, states: map[string]bool{}, stop: make(chan struct{})}
	e.SetSettings(s)
	go e.work()
	go e.watch()
	return e
}

func (e *Engine) Driver() input.Driver { return e.drv }

func (e *Engine) Settings() Settings { return *e.settings.Load() }

func (e *Engine) SetSettings(s Settings) {
	if s.TapMs < 10 {
		s.TapMs = 10
	}
	if s.ComboGapMs < 0 {
		s.ComboGapMs = 0
	}
	switch s.FocusMode {
	case FocusStrict, FocusLocal, FocusAlways, FocusOff:
	default:
		s.FocusMode = FocusLocal
	}
	e.settings.Store(&s)
}

// Close lets go of every key and stops the worker.
func (e *Engine) Close() {
	e.ReleaseAll(ReasonReleaseAll)
	close(e.stop)
}

func (e *Engine) work() {
	for {
		select {
		case job := <-e.jobs:
			job()
		case <-e.stop:
			return
		}
	}
}

// do runs fn on the worker and waits for it.
func (e *Engine) do(fn func()) {
	done := make(chan struct{})
	e.jobs <- func() { fn(); close(done) }
	<-done
}

// watch lets go of holds when the game loses focus or a hold runs too long, so a key
// can't stay down while the player types somewhere else.
func (e *Engine) watch() {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-e.stop:
			return
		case <-t.C:
		}
		if e.active.Load() == 0 {
			continue
		}
		s := e.Settings()
		lost := s.ReleaseOnFocusLoss && s.FocusMode != FocusOff && !e.drv.Game().Focused
		e.jobs <- func() {
			for _, h := range e.holds {
				switch {
				case lost:
					e.endHold(h, ReasonFocusLost)
				case s.MaxHoldMs > 0 && time.Since(h.Started) > time.Duration(s.MaxHoldMs)*time.Millisecond:
					e.endHold(h, ReasonMaxHold)
				}
			}
		}
	}
}

// admit decides whether a press may reach the keyboard, bringing the game to front
// first when the focus mode says so. Runs on the worker, right before the keys.
func (e *Engine) admit(src Source) Result {
	s := e.Settings()
	if s.FocusMode == FocusOff {
		return Result{OK: true}
	}
	g := e.drv.Game()
	if g.Focused {
		return Result{OK: true}
	}
	if !g.Running {
		return Result{Reason: ReasonNotRunning}
	}
	if s.FocusMode == FocusAlways || (s.FocusMode == FocusLocal && src.Local) {
		if e.drv.BringToFront(400 * time.Millisecond) {
			time.Sleep(time.Duration(s.SettleMs) * time.Millisecond)
			return Result{OK: true}
		}
		return Result{Reason: ReasonFocusFailed}
	}
	return Result{Reason: ReasonNotFocused}
}

func (e *Engine) gap() {
	if g := e.Settings().ComboGapMs; g > 0 {
		time.Sleep(time.Duration(g) * time.Millisecond)
	}
}

func (e *Engine) keyDown(sc keys.Scan) {
	if e.down[sc] == 0 {
		e.drv.Key(sc, true)
	}
	e.down[sc]++
}

func (e *Engine) keyUp(sc keys.Scan) {
	switch n := e.down[sc]; {
	case n <= 0:
		return
	case n == 1:
		delete(e.down, sc)
		e.drv.Key(sc, false)
	default:
		e.down[sc] = n - 1
	}
}

// liftForeign lets go of modifiers that other buttons hold and this combo doesn't use,
// so a held Alt can't turn this tap's N into Alt+N. They go back down afterwards.
func (e *Engine) liftForeign(scans []keys.Scan) []keys.Scan {
	var lifted []keys.Scan
	for sc, n := range e.down {
		if n == 0 || !sc.IsModifier() || contains(scans, sc) {
			continue
		}
		e.drv.Key(sc, false)
		lifted = append(lifted, sc)
	}
	return lifted
}

func (e *Engine) restore(lifted []keys.Scan) {
	for _, sc := range lifted {
		if e.down[sc] > 0 {
			e.drv.Key(sc, true)
		}
	}
}

func contains(list []keys.Scan, sc keys.Scan) bool {
	for _, s := range list {
		if s == sc {
			return true
		}
	}
	return false
}

func (e *Engine) pressSeq(scans []keys.Scan) {
	for i, sc := range scans {
		if i > 0 {
			e.gap()
		}
		e.keyDown(sc)
	}
}

func (e *Engine) releaseSeq(scans []keys.Scan) {
	for i := len(scans) - 1; i >= 0; i-- {
		e.keyUp(scans[i])
		if i > 0 {
			e.gap()
		}
	}
}

func (e *Engine) report(b *layout.Button, src Source, kind string, r Result) {
	if r.OK {
		e.presses.Add(1)
		e.applyState(b)
	} else {
		e.blocked.Add(1)
	}
	e.Bus.Publish("press", map[string]any{"id": b.ID, "label": b.Label, "keys": b.Keys, "kind": kind,
		"src": src, "ok": r.OK, "reason": r.Reason})
}

// Tap presses a button's keys for ms (0 = the button's own length or the tap length)
// and lets go. Short taps finish before Tap returns; long ones (timed buttons) are
// released by a timer so they don't hold up other presses.
func (e *Engine) Tap(b *layout.Button, src Source, ms int, kind string) Result {
	if ms <= 0 {
		ms = b.Ms
	}
	if ms <= 0 {
		ms = e.Settings().TapMs
	}
	if kind == "" {
		kind = "tap"
	}
	var r Result
	e.do(func() {
		if r = e.admit(src); !r.OK {
			return
		}
		lifted := e.liftForeign(b.Scans)
		e.pressSeq(b.Scans)
		if ms <= 150 {
			time.Sleep(time.Duration(ms) * time.Millisecond)
			e.releaseSeq(b.Scans)
			e.restore(lifted)
			return
		}
		h := e.newHold(b, src)
		h.lifted = lifted
		time.AfterFunc(time.Duration(ms)*time.Millisecond, func() {
			e.jobs <- func() { e.finishHold(h) }
		})
	})
	e.report(b, src, kind, r)
	return r
}

func (e *Engine) newHold(b *layout.Button, src Source) *Hold {
	e.nextID++
	h := &Hold{ID: e.nextID, Button: b, Src: src, Started: time.Now(), scans: b.Scans}
	e.holds[h.ID] = h
	e.active.Add(1)
	return h
}

func (e *Engine) finishHold(h *Hold) {
	if h.released {
		return
	}
	h.released = true
	delete(e.holds, h.ID)
	e.active.Add(-1)
	e.releaseSeq(h.scans)
	e.restore(h.lifted)
}

// endHold ends a hold the owner didn't let go of, and tells the owner.
func (e *Engine) endHold(h *Hold, reason string) {
	if h.released {
		return
	}
	e.finishHold(h)
	e.Bus.Publish("released", map[string]any{"id": h.Button.ID, "label": h.Button.Label, "reason": reason, "src": h.Src})
	if h.OnRelease != nil {
		h.OnRelease(reason)
	}
}

// Down starts holding a button. Call Up with the returned hold to let go; a nil hold
// means the press was blocked.
func (e *Engine) Down(b *layout.Button, src Source, onRelease func(reason string)) (*Hold, Result) {
	var h *Hold
	var r Result
	e.do(func() {
		if r = e.admit(src); !r.OK {
			return
		}
		h = e.newHold(b, src)
		h.OnRelease = onRelease
		h.lifted = e.liftForeign(b.Scans)
		e.pressSeq(b.Scans)
	})
	e.report(b, src, "down", r)
	return h, r
}

// Up lets go of a hold. Releases always go through, whatever the focus.
func (e *Engine) Up(h *Hold) {
	if h == nil {
		return
	}
	e.do(func() { e.finishHold(h) })
}

// ReleaseAll lets go of every held key.
func (e *Engine) ReleaseAll(reason string) {
	e.do(func() {
		for _, h := range e.holds {
			e.endHold(h, reason)
		}
		for sc := range e.down {
			e.drv.Key(sc, false)
		}
		e.down = map[keys.Scan]int{}
	})
}

// HeldKeys counts keys the engine has down, for tests and the control panel.
func (e *Engine) HeldKeys() int {
	n := 0
	e.do(func() { n = len(e.down) })
	return n
}

// Counts returns presses that went through and presses that were blocked.
func (e *Engine) Counts() (int64, int64) { return e.presses.Load(), e.blocked.Load() }

// ---------- lit buttons ----------

// The deck can't see the game, so a state is only what the deck pressed: it starts
// off, and players can reset it.
func (e *Engine) applyState(b *layout.Button) {
	if b.State == nil {
		return
	}
	e.stMu.Lock()
	on := !e.states[b.State.Group]
	if b.State.Set != nil {
		on = *b.State.Set
	}
	if on {
		e.states[b.State.Group] = true
	} else {
		delete(e.states, b.State.Group)
	}
	e.stMu.Unlock()
	e.Bus.Publish("state", map[string]any{"group": b.State.Group, "on": on})
}

// States returns every group that's on.
func (e *Engine) States() map[string]bool {
	e.stMu.Lock()
	defer e.stMu.Unlock()
	out := make(map[string]bool, len(e.states))
	for k, v := range e.states {
		out[k] = v
	}
	return out
}

// SetState sets a group by hand (from an integration that knows better).
func (e *Engine) SetState(group string, on bool) {
	e.stMu.Lock()
	if on {
		e.states[group] = true
	} else {
		delete(e.states, group)
	}
	e.stMu.Unlock()
	e.Bus.Publish("state", map[string]any{"group": group, "on": on})
}

// ResetStates turns every light off.
func (e *Engine) ResetStates() {
	e.stMu.Lock()
	e.states = map[string]bool{}
	e.stMu.Unlock()
	e.Bus.Publish("states", map[string]any{"states": map[string]bool{}})
}
