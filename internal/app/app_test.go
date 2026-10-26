package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/xattribution/tablet-deck-spooned/internal/config"
	"github.com/xattribution/tablet-deck-spooned/internal/input"
)

var nextPort = 47800

// startApp runs a deck on fresh ports with a fake keyboard.
func startApp(t *testing.T, name string) (*App, *input.Fake) {
	t.Helper()
	nextPort += 2
	f := input.NewFake()
	dir, _ := os.MkdirTemp("", "td-test")
	t.Cleanup(func() { time.Sleep(50 * time.Millisecond); os.RemoveAll(dir) })
	a, err := New(Options{DataDir: dir, Port: nextPort, Driver: f, Log: log.New(io.Discard, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	_ = a.Store.Update(func(c *config.Config) { c.Name = name; c.Voice = false; c.Discovery = false; c.TapMs = 20 })
	ctx, cancel := context.WithCancel(context.Background())
	if err := a.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Stop(); cancel() })
	return a, f
}

type wsClient struct {
	t    *testing.T
	conn *websocket.Conn
	msgs chan map[string]any
}

func dial(t *testing.T, url string) *wsClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, resp, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		t.Fatalf("dial %s: %v (%d)", url, err, code)
	}
	c := &wsClient{t: t, conn: conn, msgs: make(chan map[string]any, 100)}
	go func() {
		for {
			_, b, err := conn.Read(context.Background())
			if err != nil {
				close(c.msgs)
				return
			}
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			c.msgs <- m
		}
	}()
	go func() { // keep alive like a deck
		for range time.Tick(time.Second) {
			if conn.Write(context.Background(), websocket.MessageText, []byte(`{"t":"ping"}`)) != nil {
				return
			}
		}
	}()
	t.Cleanup(func() { conn.CloseNow() })
	return c
}

func (c *wsClient) send(v any) {
	b, _ := json.Marshal(v)
	if err := c.conn.Write(context.Background(), websocket.MessageText, b); err != nil {
		c.t.Fatal(err)
	}
}

// wait returns the first message of type typ (and matching ok, when given).
func (c *wsClient) wait(typ string, ok func(map[string]any) bool) map[string]any {
	c.t.Helper()
	timeout := time.After(4 * time.Second)
	for {
		select {
		case m, open := <-c.msgs:
			if !open {
				c.t.Fatalf("connection closed waiting for %s", typ)
			}
			if m["t"] == typ && (ok == nil || ok(m)) {
				return m
			}
		case <-timeout:
			c.t.Fatalf("no %s message", typ)
		}
	}
}

func TestOwnerDeckPairsAndPresses(t *testing.T) {
	a, f := startApp(t, "Jared")
	cfg := a.Store.Config()
	// A device that used the pairing code gets its own token...
	c := dial(t, fmt.Sprintf("ws://%s:%d/ws?t=%s&device=tablet", a.LanAddress(), cfg.Port, cfg.Token))
	paired := c.wait("paired", nil)
	tok := paired["token"].(string)
	c.wait("hello", nil)
	c.send(map[string]any{"t": "tap", "id": "pl-gear"})
	c.wait("state", func(m map[string]any) bool { return m["group"] == "gear" && m["on"] == true })
	if evs := f.Events(); len(evs) != 2 {
		t.Fatalf("want gear down+up, got %d events", len(evs))
	}
	// ...which keeps working after the pairing code changes.
	_ = a.Store.Update(func(c *config.Config) { c.Token = "newcode1" })
	c2 := dial(t, fmt.Sprintf("ws://%s:%d/ws?t=%s", a.LanAddress(), cfg.Port, tok))
	c2.wait("hello", nil)

	// Unpairing disconnects it.
	a.unpair(paired["id"].(string))
	c2.wait("unpaired", nil)
	resp, _ := http.Get(fmt.Sprintf("http://%s:%d/layout.json?t=%s", a.LanAddress(), cfg.Port, tok))
	if resp.StatusCode != 403 {
		t.Fatalf("unpaired token still works: %d", resp.StatusCode)
	}
}

func TestGuestLinkScope(t *testing.T) {
	a, f := startApp(t, "Jared")
	cfg := a.Store.Config()
	g, err := a.CreateGrant(GrantSpec{Label: "Mike", Panels: []string{"power-landing"}, Minutes: 30})
	if err != nil {
		t.Fatal(err)
	}
	owner := dial(t, fmt.Sprintf("ws://127.0.0.1:%d/ws?t=%s", cfg.Port, cfg.Token))
	owner.wait("hello", nil)

	// The guest port (where a tunnel points) serves the share link but nothing else.
	guest := dial(t, fmt.Sprintf("ws://127.0.0.1:%d/gws?g=%s", cfg.GuestPort, g.Token))
	hello := guest.wait("hello", nil)
	allowed := hello["guest"].(map[string]any)["allowed"].([]any)
	if len(allowed) == 0 {
		t.Fatal("guest sees no buttons")
	}
	f.Events()
	guest.send(map[string]any{"t": "tap", "id": "pl-gear"})
	owner.wait("toast", func(m map[string]any) bool { return strings.Contains(m["text"].(string), "Mike") })
	if len(f.Events()) != 2 {
		t.Fatal("guest press didn't reach the keyboard")
	}
	guest.send(map[string]any{"t": "tap", "id": "top-starmap"}) // not shared
	if m := guest.wait("blocked", nil); m["reason"] != "denied" {
		t.Fatalf("blocked for %v", m["reason"])
	}
	if len(f.Events()) != 0 {
		t.Fatal("unshared button was pressed")
	}

	// Guests can ask for what they can't press; the owner's "Do it" presses it.
	guest.send(map[string]any{"t": "request", "id": "top-starmap", "text": "open your map pls"})
	req := owner.wait("request", nil)["request"].(map[string]any)
	owner.send(map[string]any{"t": "answer", "id": req["id"], "do": true})
	if m := guest.wait("answered", nil); m["result"] != "done" {
		t.Fatalf("answer %v", m)
	}
	if len(f.Events()) != 2 {
		t.Fatal("answered request didn't press")
	}

	// Owner routes are closed on the guest port, even with the pairing code.
	for _, p := range []string{"/control", "/api/v1/status", "/editor/layouts", "/layout.json?t=" + cfg.Token} {
		resp, _ := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s", cfg.GuestPort, p))
		if resp.StatusCode < 400 {
			t.Fatalf("guest port answered %s with %d", p, resp.StatusCode)
		}
	}

	a.RevokeGrant(g.ID)
	guest.wait("revoked", nil)
}

func TestAfkOnlyLink(t *testing.T) {
	a, _ := startApp(t, "Jared")
	cfg := a.Store.Config()
	g, _ := a.CreateGrant(GrantSpec{Label: "Wingman", All: true, AfkOnly: true})
	guest := dial(t, fmt.Sprintf("ws://127.0.0.1:%d/gws?g=%s", cfg.Port, g.Token))
	guest.wait("hello", nil)
	guest.send(map[string]any{"t": "tap", "id": "pl-gear"})
	if m := guest.wait("blocked", nil); m["reason"] != "not-afk" {
		t.Fatalf("reason %v", m["reason"])
	}
	a.SetAfk(true)
	guest.wait("afk", nil)
	guest.send(map[string]any{"t": "tap", "id": "pl-gear"})
	guest.wait("state", func(m map[string]any) bool { return m["group"] == "gear" })
}

// Two PCs: Mike links to Jared's share link, then presses Jared's gear from his own
// deck and sends him a request; Jared answers.
func TestCrewLink(t *testing.T) {
	jared, jf := startApp(t, "Jared")
	mike, _ := startApp(t, "Mike")
	g, _ := jared.CreateGrant(GrantSpec{Label: "Mike", Panels: []string{"power-landing"}})
	link := jared.GrantLinks(g)["lan"]
	link = strings.Replace(link, jared.LanAddress(), "127.0.0.1", 1)
	if _, err := mike.crew.Add("Jared", link); err != nil {
		t.Fatal(err)
	}
	mcfg, jcfg := mike.Store.Config(), jared.Store.Config()
	mikeDeck := dial(t, fmt.Sprintf("ws://127.0.0.1:%d/ws?t=%s", mcfg.Port, mcfg.Token))
	mikeDeck.wait("hello", nil)
	crew := mikeDeck.wait("crew", func(m map[string]any) bool {
		list, _ := m["crew"].([]any)
		return len(list) > 0 && list[0].(map[string]any)["online"] == true
	})["crew"].([]any)[0].(map[string]any)
	if btns, _ := crew["buttons"].([]any); len(btns) == 0 {
		t.Fatal("crew link shows no buttons")
	}
	jaredDeck := dial(t, fmt.Sprintf("ws://127.0.0.1:%d/ws?t=%s", jcfg.Port, jcfg.Token))
	jaredDeck.wait("hello", nil)
	jf.Events()

	mikeDeck.send(map[string]any{"t": "crew-press", "peer": crew["id"], "id": "pl-gear"})
	jaredDeck.wait("state", func(m map[string]any) bool { return m["group"] == "gear" })
	if len(jf.Events()) != 2 {
		t.Fatal("crew press didn't reach Jared's keyboard")
	}

	mikeDeck.send(map[string]any{"t": "crew-request", "peer": crew["id"], "text": "brb, keep us out of the asteroid"})
	req := jaredDeck.wait("request", nil)["request"].(map[string]any)
	if !strings.Contains(req["from"].(string), "Mike") {
		t.Fatalf("request from %v", req["from"])
	}

	// And back: Jared sends Mike a request down the link Mike opened.
	jcrew := jared.crew.summary()
	if len(jcrew) == 0 {
		t.Fatal("Jared doesn't see Mike connected")
	}
	jaredDeck.send(map[string]any{"t": "crew-request", "peer": jcrew[0]["id"], "text": "on it"})
	back := mikeDeck.wait("request", nil)["request"].(map[string]any)
	if back["from"] != "Jared" || back["text"] != "on it" {
		t.Fatalf("request back: %v", back)
	}
}

func TestStaleDeckReleasesKeys(t *testing.T) {
	a, f := startApp(t, "Jared")
	cfg := a.Store.Config()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/ws?t=%s", cfg.Port, cfg.Token), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Find a hold button and hold it, then go silent like a tablet that fell asleep.
	l, _ := a.Layout("tablet", "")
	hold := ""
	for _, id := range l.Order {
		if l.Buttons[id].Mode == "hold" {
			hold = id
			break
		}
	}
	if hold == "" {
		t.Skip("no hold button in the layout")
	}
	_ = conn.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf(`{"t":"down","id":%q}`, hold)))
	deadline := time.Now().Add(time.Second)
	for a.Engine.HeldKeys() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if a.Engine.HeldKeys() == 0 {
		t.Fatal("hold didn't press")
	}
	deadline = time.Now().Add(9 * time.Second)
	for a.Engine.HeldKeys() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("keys still down after the deck went silent")
		}
		time.Sleep(100 * time.Millisecond)
	}
	evs := f.Events()
	if len(evs) == 0 || evs[len(evs)-1].Down {
		t.Fatal("last key event wasn't a release")
	}
}
