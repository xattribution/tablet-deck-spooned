package app

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

// Three tablets make one cockpit: each shows part of the view; a panel moves between
// them; when one drops off its panels go to a neighbor and come back with it.
func TestCockpit(t *testing.T) {
	consoleGrace = 150 * time.Millisecond
	a, _ := startApp(t, "Pilot")
	cfg := a.Store.Config()
	url := func(tok string) string {
		return fmt.Sprintf("ws://%s:%d/ws?t=%s&device=tablet", a.LanAddress(), cfg.Port, tok)
	}
	var decks []*wsClient
	var toks []string
	for i := 0; i < 3; i++ {
		c := dial(t, url(cfg.Token))
		toks = append(toks, c.wait("paired", nil)["token"].(string))
		c.wait("hello", nil)
		decks = append(decks, c)
	}
	time.Sleep(100 * time.Millisecond)
	shares := func() [][]string {
		st := a.CockpitStatus()
		var out [][]string
		for _, con := range st["consoles"].([]map[string]any) {
			var ids []string
			for _, p := range con["panels"].([]map[string]any) {
				ids = append(ids, p["id"].(string))
			}
			out = append(out, ids)
		}
		return out
	}
	s := shares()
	all := []string{}
	for _, p := range s {
		if len(p) == 0 {
			t.Fatalf("a console got nothing: %v", s)
		}
		all = append(all, p...)
	}
	flight, _ := a.ViewByID("flight")
	sort.Strings(all)
	want := append([]string(nil), flight.Groups...)
	sort.Strings(want)
	if strings.Join(all, ",") != strings.Join(want, ",") {
		t.Fatalf("the view isn't shared out whole: %v vs %v", all, want)
	}

	// Each deck's latest view message has just its share.
	last := func(c *wsClient) []string {
		var ids []string
		deadline := time.After(time.Second)
		for {
			select {
			case m := <-c.msgs:
				if m["t"] == "view" {
					ids = nil
					for _, g := range m["groups"].([]any) {
						ids = append(ids, g.(map[string]any)["id"].(string))
					}
				}
			case <-deadline:
				return ids
			}
		}
	}

	// Move the first console's first panel to the third.
	st := a.CockpitStatus()["consoles"].([]map[string]any)
	moved := s[0][0]
	if err := a.MovePanel(moved, st[2]["id"].(string), ""); err != nil {
		t.Fatal(err)
	}
	if got := shares(); !contains(got[2], moved) || contains(got[0], moved) {
		t.Fatalf("panel didn't move: %v", got)
	}

	// The middle console drops off: after the grace its panels show on a neighbor.
	mid := shares()[1]
	decks[1].conn.CloseNow()
	time.Sleep(400 * time.Millisecond)
	lostSeen := false
	for !lostSeen {
		select {
		case m := <-decks[0].msgs:
			lostSeen = m["t"] == "cockpit-lost"
		case <-time.After(time.Second):
			t.Fatal("no cockpit-lost message")
		}
	}
	got0, got2 := last(decks[0]), last(decks[2])
	for _, g := range mid {
		if !contains(got0, g) && !contains(got2, g) {
			t.Fatalf("%s isn't shown anywhere after its console dropped: %v %v", g, got0, got2)
		}
	}

	// It comes back (same device): its panels go home.
	back := dial(t, url(toks[1]))
	back.wait("hello", nil)
	if got := last(back); len(got) != len(mid) {
		t.Fatalf("returning console shows %v, want %v", got, mid)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// A tablet that joins after the split was saved still gets its share, and a saved
// place on a console that isn't here this session doesn't keep panels from it.
func TestCockpitLateJoiner(t *testing.T) {
	consoleGrace = 150 * time.Millisecond
	a, _ := startApp(t, "Pilot")
	cfg := a.Store.Config()
	url := fmt.Sprintf("ws://%s:%d/ws?t=%s&device=tablet", a.LanAddress(), cfg.Port, cfg.Token)
	join := func() *wsClient {
		c := dial(t, url)
		c.wait("paired", nil)
		c.wait("hello", nil)
		return c
	}
	count := func() []int {
		var n []int
		for _, con := range a.CockpitStatus()["consoles"].([]map[string]any) {
			if con["online"].(bool) {
				n = append(n, len(con["panels"].([]map[string]any)))
			}
		}
		return n
	}
	join()
	b := join()
	time.Sleep(100 * time.Millisecond)
	st := a.CockpitStatus()["consoles"].([]map[string]any)
	first := st[0]["panels"].([]map[string]any)[0]["id"].(string)
	if err := a.MovePanel(first, st[1]["id"].(string), ""); err != nil { // saves the split
		t.Fatal(err)
	}
	join()
	time.Sleep(100 * time.Millisecond)
	for _, n := range count() {
		if n == 0 {
			t.Fatalf("the late joiner got nothing: %v", count())
		}
	}
	// One leaves for good (past the grace) and isn't lost any more after a restart of
	// the session: its saved place must not hold panels the others can't see.
	b.conn.CloseNow()
	time.Sleep(400 * time.Millisecond)
	a.cockpit.mu.Lock()
	for k := range a.cockpit.lost {
		delete(a.cockpit.lost, k)
	}
	a.cockpit.mu.Unlock()
	total := 0
	for _, n := range count() {
		total += n
	}
	flight, _ := a.ViewByID("flight")
	if total != len(flight.Groups) {
		t.Fatalf("panels stuck on an absent console: %d of %d shown (%v)", total, len(flight.Groups), count())
	}
}
