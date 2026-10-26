package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/xattribution/tablet-deck-spooned/internal/config"
)

// crew keeps this deck linked to crewmates' decks. A link is a share link another deck
// gave us: we connect to it as a guest (marked "peer"), so our tablets can press the
// buttons it covers and send requests. The same connection carries requests back.
type crew struct {
	a     *App
	ctx   context.Context
	mu    sync.Mutex
	peers map[string]*peerConn // by CrewLink ID
}

type peerConn struct {
	link   config.CrewLink
	online bool
	hello  map[string]any
	afk    bool
	out    chan []byte
	cancel context.CancelFunc
}

func newCrew(a *App) *crew { return &crew{a: a, peers: map[string]*peerConn{}} }

func (cr *crew) start(ctx context.Context) {
	cr.ctx = ctx
	for _, l := range cr.a.Store.State().Crew {
		cr.connect(l)
	}
}

// ParseShareLink turns a share link into the WebSocket address of its deck.
func ParseShareLink(link string) (wsURL string, err error) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("that isn't a Tablet Deck share link")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "g" || len(parts[1]) < 20 {
		return "", fmt.Errorf("that isn't a Tablet Deck share link (it should end in /g/...)")
	}
	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	return fmt.Sprintf("%s://%s/gws?g=%s", scheme, u.Host, url.QueryEscape(parts[1])), nil
}

// Add links to a crewmate's deck by their share link.
func (cr *crew) Add(name, link string) (config.CrewLink, error) {
	if _, err := ParseShareLink(link); err != nil {
		return config.CrewLink{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Crewmate"
	}
	l := config.CrewLink{ID: config.RandomHex(5), Name: name, URL: strings.TrimSpace(link), Added: time.Now()}
	if err := cr.a.Store.UpdateState(func(s *config.State) { s.Crew = append(s.Crew, l) }); err != nil {
		return l, err
	}
	cr.connect(l)
	return l, nil
}

// Remove drops a crew link.
func (cr *crew) Remove(id string) {
	_ = cr.a.Store.UpdateState(func(s *config.State) {
		kept := s.Crew[:0]
		for _, l := range s.Crew {
			if l.ID != id {
				kept = append(kept, l)
			}
		}
		s.Crew = kept
	})
	cr.mu.Lock()
	if p := cr.peers[id]; p != nil {
		p.cancel()
		delete(cr.peers, id)
	}
	cr.mu.Unlock()
	cr.changed()
}

func (cr *crew) connect(l config.CrewLink) {
	ctx, cancel := context.WithCancel(cr.ctx)
	p := &peerConn{link: l, out: make(chan []byte, 64), cancel: cancel}
	cr.mu.Lock()
	cr.peers[l.ID] = p
	cr.mu.Unlock()
	go cr.run(ctx, p)
}

// run keeps one crew link connected, retrying with backoff.
func (cr *crew) run(ctx context.Context, p *peerConn) {
	wsURL, err := ParseShareLink(p.link.URL)
	if err != nil {
		return
	}
	backoff := time.Second
	for ctx.Err() == nil {
		name := url.QueryEscape(cr.a.Store.Config().Name)
		dctx, dcancel := context.WithTimeout(ctx, 10*time.Second)
		conn, resp, err := websocket.Dial(dctx, wsURL+"&peer="+name, nil)
		if conn != nil {
			conn.SetReadLimit(8 << 20) // a deck's hello carries its panels
		}
		dcancel()
		if err != nil {
			if resp != nil && (resp.StatusCode == 403 || resp.StatusCode == 410) {
				cr.setOnline(p, false, nil)
				cr.a.broadcastOwners(map[string]any{"t": "toast", "text": p.link.Name + "'s link was revoked or ran out"})
				backoff = 5 * time.Minute
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < time.Minute {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		cr.session(ctx, p, conn)
		cr.setOnline(p, false, nil)
	}
}

func (cr *crew) session(ctx context.Context, p *peerConn, conn *websocket.Conn) {
	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer conn.CloseNow()
	// Presses queued while the link was down are stale now: drop them, don't replay.
	for len(p.out) > 0 {
		<-p.out
	}
	go func() {
		t := time.NewTicker(1500 * time.Millisecond)
		defer t.Stop()
		for {
			var b []byte
			select {
			case <-sctx.Done():
				return
			case <-t.C:
				b = []byte(`{"t":"ping"}`)
			case b = <-p.out:
			}
			wctx, wc := context.WithTimeout(sctx, 5*time.Second)
			err := conn.Write(wctx, websocket.MessageText, b)
			wc()
			if err != nil {
				cancel()
				return
			}
		}
	}()
	for {
		rctx, rc := context.WithTimeout(sctx, 8*time.Second)
		_, data, err := conn.Read(rctx)
		rc()
		if err != nil {
			return
		}
		var m map[string]any
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		cr.fromPeer(p, m)
	}
}

// fromPeer handles what a linked deck sends us.
func (cr *crew) fromPeer(p *peerConn, m map[string]any) {
	a := cr.a
	switch m["t"] {
	case "hello":
		cr.setOnline(p, true, m)
	case "afk":
		on, _ := m["on"].(bool)
		cr.mu.Lock()
		p.afk = on
		cr.mu.Unlock()
		cr.changed()
		if on {
			a.broadcastOwners(map[string]any{"t": "toast", "text": p.link.Name + " is AFK"})
		}
	case "blocked":
		reason, _ := m["reason"].(string)
		a.broadcastOwners(map[string]any{"t": "toast", "text": fmt.Sprintf("%s's deck didn't press it (%s)", p.link.Name, reason)})
	case "answered":
		res, _ := m["result"].(string)
		label, _ := m["label"].(string)
		if label == "" {
			label, _ = m["text"].(string)
		}
		a.broadcastOwners(map[string]any{"t": "toast", "text": fmt.Sprintf("%s: %s %s", p.link.Name, label, res)})
	case "crew-request":
		text, _ := m["text"].(string)
		from, _ := m["from"].(string)
		if from == "" {
			from = p.link.Name
		}
		a.peerRequest(from, text, func(v map[string]any) { cr.sendTo(p, v) })
	case "revoked":
		a.broadcastOwners(map[string]any{"t": "toast", "text": p.link.Name + "'s link was revoked"})
	}
}

func (cr *crew) sendTo(p *peerConn, v any) {
	b, _ := json.Marshal(v)
	select {
	case p.out <- b:
	default:
	}
}

func (cr *crew) setOnline(p *peerConn, on bool, hello map[string]any) {
	cr.mu.Lock()
	p.online = on
	if hello != nil {
		p.hello = hello
		p.afk, _ = hello["afk"].(bool)
	}
	cr.mu.Unlock()
	cr.changed()
}

func (cr *crew) changed() {
	cr.a.broadcastOwners(map[string]any{"t": "crew", "crew": cr.summary()})
	cr.a.Bus.Publish("crew", nil)
}

// summary is what the owner's decks show in their Crew sheet: decks we're linked to
// (with the buttons they share with us) and crewmates connected to us.
func (cr *crew) summary() []map[string]any {
	out := []map[string]any{}
	if cr == nil {
		return out
	}
	cr.mu.Lock()
	for id, p := range cr.peers {
		e := map[string]any{"id": "out:" + id, "name": p.link.Name, "online": p.online, "afk": p.afk, "linked": true}
		if g, ok := p.hello["guest"].(map[string]any); ok {
			e["buttons"] = g["buttons"]
			e["expires"] = g["expires"]
			e["afkOnly"] = g["afkOnly"]
		}
		if n, ok := p.hello["name"].(string); ok {
			e["deck"] = n
		}
		out = append(out, e)
	}
	cr.mu.Unlock()
	cr.a.each(func(c *client) {
		if c.p.grant != nil {
			out = append(out, map[string]any{"id": "in:" + c.id, "name": c.name(), "online": true, "guest": true, "peer": c.peer})
		}
	})
	return out
}

func (cr *crew) find(peer string) (*peerConn, *client) {
	if id, ok := strings.CutPrefix(peer, "out:"); ok {
		cr.mu.Lock()
		defer cr.mu.Unlock()
		return cr.peers[id], nil
	}
	if id, ok := strings.CutPrefix(peer, "in:"); ok {
		var found *client
		cr.a.each(func(c *client) {
			if c.id == id && c.p.grant != nil {
				found = c
			}
		})
		return nil, found
	}
	return nil, nil
}

// press presses a button on a linked crewmate's deck.
func (cr *crew) press(peer, button string, from *client) {
	p, _ := cr.find(peer)
	if p == nil || !p.online {
		from.send(map[string]any{"t": "toast", "text": "That crewmate isn't connected"})
		return
	}
	cr.sendTo(p, map[string]any{"t": "tap", "id": button})
}

// request sends a crewmate a request: through our link to their deck, or down the
// connection they made to ours.
func (cr *crew) request(peer, text, button string, from *client) {
	text = strings.TrimSpace(text)
	if len(text) > 140 {
		text = text[:140]
	}
	p, c := cr.find(peer)
	switch {
	case p != nil && p.online:
		cr.sendTo(p, map[string]any{"t": "request", "text": text, "id": button})
	case c != nil:
		c.send(map[string]any{"t": "crew-request", "from": cr.a.Store.Config().Name, "text": text})
	default:
		from.send(map[string]any{"t": "toast", "text": "That crewmate isn't connected"})
		return
	}
	from.send(map[string]any{"t": "toast", "text": "Sent"})
}

// peerRequest shows a request that came from a deck we're linked to.
func (a *App) peerRequest(from, text string, reply func(map[string]any)) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if len(text) > 140 {
		text = text[:140]
	}
	q := &request{ID: config.RandomHex(5), From: from, Text: text, Time: time.Now(), reply: reply}
	a.reqs.mu.Lock()
	a.reqs.list = append(a.reqs.list, q)
	a.reqs.mu.Unlock()
	a.broadcastOwners(map[string]any{"t": "request", "request": q})
	a.Bus.Publish("request", q)
	a.voice.say(from + " says: " + text)
}
