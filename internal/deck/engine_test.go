package deck

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xattribution/tablet-deck-spooned/internal/input"
	"github.com/xattribution/tablet-deck-spooned/internal/keys"
	"github.com/xattribution/tablet-deck-spooned/internal/layout"
)

func btn(t *testing.T, id, combo string) *layout.Button {
	t.Helper()
	sc, err := keys.ParseCombo(combo)
	if err != nil {
		t.Fatal(err)
	}
	return &layout.Button{ID: id, Label: id, Keys: combo, Scans: sc}
}

func names(evs []input.KeyEvent) string {
	rev := map[keys.Scan]string{}
	for _, n := range keys.Names() {
		s, _ := keys.Lookup(n)
		rev[s] = n
	}
	var parts []string
	for _, e := range evs {
		d := "up"
		if e.Down {
			d = "down"
		}
		parts = append(parts, fmt.Sprintf("%s:%s", rev[e.Scan], d))
	}
	return strings.Join(parts, " ")
}

func newEngine(t *testing.T) (*Engine, *input.Fake) {
	t.Helper()
	f := input.NewFake()
	s := DefaultSettings()
	s.ComboGapMs = 0
	s.TapMs = 5
	e := New(f, s, NewBus(0))
	t.Cleanup(e.Close)
	return e, f
}

func TestTapCombo(t *testing.T) {
	e, f := newEngine(t)
	if r := e.Tap(btn(t, "atc", "lalt+n"), Source{Kind: "deck"}, 0, ""); !r.OK {
		t.Fatalf("tap blocked: %+v", r)
	}
	if got, want := names(f.Events()), "lalt:down n:down n:up lalt:up"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if e.HeldKeys() != 0 {
		t.Fatal("keys left down")
	}
}

func TestSharedModifierRefcount(t *testing.T) {
	e, f := newEngine(t)
	a, _ := e.Down(btn(t, "a", "lshift+w"), Source{}, nil)
	b, _ := e.Down(btn(t, "b", "lshift+s"), Source{}, nil)
	e.Up(a)
	// lshift is still held by b, so it must not come up yet.
	if got := names(f.Events()); strings.Count(got, "lshift:up") != 0 {
		t.Fatalf("shift released early: %s", got)
	}
	e.Up(b)
	if got := names(f.Events()); got != "s:up lshift:up" {
		t.Fatalf("got %q", got)
	}
}

func TestModifierIsolation(t *testing.T) {
	e, f := newEngine(t)
	h, _ := e.Down(btn(t, "look", "lalt+z"), Source{}, nil)
	f.Events()
	e.Tap(btn(t, "gear", "n"), Source{}, 0, "")
	// Alt is lifted around the tap so the game sees N, not Alt+N, then pressed again.
	if got, want := names(f.Events()), "lalt:up n:down n:up lalt:down"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	e.Up(h)
	if e.HeldKeys() != 0 {
		t.Fatal("keys left down")
	}
}

func TestFocusModes(t *testing.T) {
	e, f := newEngine(t)
	gear := btn(t, "gear", "n")
	local, remote := Source{Local: true}, Source{}

	f.SetGame(false, false, false)
	if r := e.Tap(gear, remote, 0, ""); r.OK || r.Reason != ReasonNotRunning {
		t.Fatalf("not running: %+v", r)
	}

	f.SetGame(true, false, true)
	if r := e.Tap(gear, remote, 0, ""); r.OK || r.Reason != ReasonNotFocused {
		t.Fatalf("remote press in background: %+v", r)
	}
	// The original's big complaint: a touchscreen on the same PC takes focus from the
	// game. Local presses bring the game back to front, then press.
	if r := e.Tap(gear, local, 0, ""); !r.OK {
		t.Fatalf("local press should refocus: %+v", r)
	}
	if !f.Game().Focused {
		t.Fatal("game not brought to front")
	}

	f.SetGame(true, false, false)
	if r := e.Tap(gear, local, 0, ""); r.OK || r.Reason != ReasonFocusFailed {
		t.Fatalf("refocus failure: %+v", r)
	}

	s := e.Settings()
	s.FocusMode = FocusStrict
	e.SetSettings(s)
	f.SetGame(true, false, true)
	if r := e.Tap(gear, local, 0, ""); r.OK {
		t.Fatal("strict mode let a background press through")
	}
	s.FocusMode = FocusOff
	e.SetSettings(s)
	if r := e.Tap(gear, remote, 0, ""); !r.OK {
		t.Fatal("focus off blocked a press")
	}
}

func TestReleaseOnFocusLoss(t *testing.T) {
	e, f := newEngine(t)
	var mu sync.Mutex
	reason := ""
	h, r := e.Down(btn(t, "boost", "lshift"), Source{}, func(why string) { mu.Lock(); reason = why; mu.Unlock() })
	if !r.OK {
		t.Fatal(r)
	}
	f.SetGame(true, false, false)
	deadline := time.Now().Add(2 * time.Second)
	for e.HeldKeys() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("hold not released after focus loss")
		}
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if reason != ReasonFocusLost {
		t.Fatalf("reason %q", reason)
	}
	e.Up(h) // the owner's late release is harmless
	if got := names(f.Events()); got != "lshift:down lshift:up" {
		t.Fatalf("got %q", got)
	}
}

func TestLongTimedPressDoesNotBlock(t *testing.T) {
	e, f := newEngine(t)
	start := time.Now()
	e.Tap(btn(t, "qt", "b"), Source{}, 400, "timed")
	e.Tap(btn(t, "gear", "n"), Source{}, 0, "")
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("a long press held up the next tap")
	}
	time.Sleep(500 * time.Millisecond)
	if got, want := names(f.Events()), "b:down n:down n:up b:up"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestReleaseAllAndStates(t *testing.T) {
	e, _ := newEngine(t)
	e.Down(btn(t, "a", "w"), Source{}, nil)
	e.Down(btn(t, "b", "lctrl+s"), Source{}, nil)
	e.ReleaseAll(ReasonReleaseAll)
	if e.HeldKeys() != 0 {
		t.Fatal("release all left keys down")
	}
	g := btn(t, "gear", "n")
	g.State = &layout.State{Group: "gear"}
	e.Tap(g, Source{}, 0, "")
	if !e.States()["gear"] {
		t.Fatal("gear state not on")
	}
	e.Tap(g, Source{}, 0, "")
	if e.States()["gear"] {
		t.Fatal("gear state not toggled off")
	}
}
