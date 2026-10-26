package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/xattribution/tablet-deck-spooned/internal/config"
	"github.com/xattribution/tablet-deck-spooned/internal/deck"
	"github.com/xattribution/tablet-deck-spooned/internal/layout"
)

// client is one connected deck: the owner's tablet or phone, a guest on a share link,
// or another PC linked as crew (a guest whose "peer" is set).
type client struct {
	a      *App
	id     string
	p      principal
	peer   bool // another Tablet Deck, not a browser
	layout *layout.Layout
	out    chan []byte
	cancel context.CancelFunc

	mu      sync.Mutex
	holds   map[string]*deck.Hold
	rate    []time.Time // guest presses in the last minute
	listens bool        // holding the talk button
	view    string      // the view this deck shows
	theme   string      // set by an agent; "" = the deck's own choice
	form    string      // tablet or phone, as the deck said

	ip         string // where it connected from
	platform   string // ipad, iphone, android, windows, mac or other
	installID  string // the Android app's install id (?did=)
	nativeApp  string // "android" when it runs in the Android app (?native=)
	nativeInfo any    // what the Android app said about itself (hello-native)
}

// msg is anything a deck sends. Fields are shared between message types.
type msg struct {
	T        string   `json:"t"`
	ID       string   `json:"id,omitempty"`
	N        float64  `json:"n,omitempty"`
	On       bool     `json:"on,omitempty"`
	Index    int      `json:"index,omitempty"`
	Text     string   `json:"text,omitempty"`
	Peer     string   `json:"peer,omitempty"`
	Mode     string   `json:"mode,omitempty"`
	Do       bool     `json:"do,omitempty"`
	Label    string   `json:"label,omitempty"`
	Mins     int      `json:"minutes,omitempty"`
	All      bool     `json:"all,omitempty"`
	AFK      bool     `json:"afkOnly,omitempty"`
	Panels   []string `json:"panels,omitempty"`
	Btns     []string `json:"buttons,omitempty"`
	Ms       int      `json:"ms,omitempty"`
	Result   string   `json:"result,omitempty"`
	Groups   []string `json:"groups,omitempty"`
	Controls []string `json:"controls,omitempty"`
	NoHold   bool     `json:"tapsOnly,omitempty"`
}

func (c *client) send(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.out <- b:
	default: // a deck that stopped reading loses messages rather than holding up the others
	}
}

// kick sends a last message and disconnects once it has had time to go out.
func (c *client) kick(v any) {
	c.send(v)
	c.releaseAll()
	time.AfterFunc(300*time.Millisecond, c.cancel)
}

func (c *client) name() string {
	if c.p.name != "" {
		return c.p.name
	}
	return "a deck"
}

func (c *client) source() deck.Source {
	switch {
	case c.peer:
		return deck.Source{Kind: "peer", Who: c.name()}
	case c.p.grant != nil:
		return deck.Source{Kind: "guest", Who: c.name()}
	}
	return deck.Source{Kind: "deck", Who: c.name(), Local: c.p.local}
}

// handleWS upgrades /ws (owners) and /gws (guests and crew) to a WebSocket.
func (a *App) handleWS(w http.ResponseWriter, r *http.Request, guestListener bool) {
	var p principal
	var status int
	guest := r.URL.Path == "/gws"
	if guest {
		p, status = a.authGuest(r)
	} else {
		p, status = a.authOwner(r, guestListener)
	}
	if status != 0 {
		http.Error(w, http.StatusText(status), status)
		return
	}
	q := r.URL.Query()
	var l *layout.Layout
	var err error
	if p.grant != nil {
		l, err = a.LayoutFile(p.grant.Layout)
		if peer := q.Get("peer"); peer != "" && len(peer) <= 40 {
			p.name = peer + " (" + p.grant.Label + ")"
		}
	} else {
		l, err = a.Layout(q.Get("device"), q.Get("layout"))
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	conn.SetReadLimit(64 << 10)
	ctx, cancel := context.WithCancel(r.Context())
	c := &client{a: a, id: config.RandomHex(6), p: p, peer: q.Get("peer") != "", layout: l, form: q.Get("device"),
		out: make(chan []byte, 256), cancel: cancel, holds: map[string]*deck.Hold{}, ip: remoteIP(r)}
	if p.grant == nil {
		c.installID = cleanID(q.Get("did"))
		if q.Get("native") == "android" {
			c.nativeApp = "android"
		}
		c.platform = platformOf(r.UserAgent(), c.form, c.nativeApp)
	}

	// A device that used the pairing code gets a token of its own, so changing the code
	// later doesn't unpair it, and it can be unpaired on its own.
	if p.owner && p.byCode && !p.local {
		d, tok := a.mintDevice(p.name)
		c.p.deviceID, c.p.byCode = d.ID, false
		c.send(map[string]any{"t": "paired", "token": tok, "id": d.ID, "name": d.Name})
	}
	a.touchDevice(c.p.deviceID)

	a.clientsMu.Lock()
	a.clients[c] = struct{}{}
	a.clientsMu.Unlock()
	a.Log.Printf("%s connected (%s).", c.describe(), remoteIP(r))
	a.Bus.Publish("clients", a.clientSummary())
	if c.p.grant != nil && a.crew != nil {
		a.crew.changed() // owners' crew lists show who's on their links
	}
	a.joinCockpit(c)
	c.send(a.helloFor(c))
	if a.inCockpit(c) {
		a.pushCockpit() // a console that arrives may change how the view is shared out
	}

	go func() { // writer
		for {
			select {
			case <-ctx.Done():
				return
			case b := <-c.out:
				wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
				err := conn.Write(wctx, websocket.MessageText, b)
				wcancel()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()

	for {
		// Decks send a ping every 1.5 s; one silent for 6 s has gone (asleep, out of
		// Wi-Fi), and its held keys are let go below.
		rctx, rcancel := context.WithTimeout(ctx, 6*time.Second)
		typ, data, err := conn.Read(rctx)
		rcancel()
		if err != nil {
			break
		}
		if typ != websocket.MessageText {
			continue
		}
		var m msg
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		c.handle(m)
	}
	cancel()
	conn.CloseNow()
	c.releaseAll()
	a.voice.stopFor(c)
	a.clientsMu.Lock()
	delete(a.clients, c)
	a.clientsMu.Unlock()
	a.leftCockpit(c)
	a.Log.Printf("%s disconnected.", c.describe())
	a.Bus.Publish("clients", a.clientSummary())
	if c.p.grant != nil && a.crew != nil {
		a.crew.changed()
	}
}

// platformOf is a coarse guess at what a deck runs on, from its browser. iPadOS
// Safari asks for the desktop site and says "Macintosh"; a deck that also says it is a
// tablet is taken for an iPad.
func platformOf(ua, form, native string) string {
	switch {
	case native == "android":
		return "android"
	case strings.Contains(ua, "iPad"):
		return "ipad"
	case strings.Contains(ua, "iPhone"):
		return "iphone"
	case strings.Contains(ua, "Android"):
		return "android"
	case strings.Contains(ua, "Macintosh") && form == "tablet":
		return "ipad"
	case strings.Contains(ua, "Macintosh"):
		return "mac"
	case strings.Contains(ua, "Windows"):
		return "windows"
	}
	return "other"
}

// cleanID keeps an id a device sent if it is short and plain.
func cleanID(s string) string {
	if len(s) == 0 || len(s) > 64 {
		return ""
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return ""
		}
	}
	return s
}

func (c *client) describe() string {
	switch {
	case c.peer:
		return "Crew link " + c.name()
	case c.p.grant != nil:
		return "Guest " + c.name()
	}
	return "Deck " + c.name()
}

func (c *client) releaseAll() {
	c.mu.Lock()
	holds := c.holds
	c.holds = map[string]*deck.Hold{}
	c.mu.Unlock()
	for _, h := range holds {
		c.a.Engine.Up(h)
	}
}

// allowed reports whether this deck may press a button: always for the owner; for a
// guest, when the share link covers it (and the owner is away, for AFK-only links).
func (c *client) allowed(b *layout.Button) (bool, string) {
	g := c.p.grant
	if g == nil {
		return true, ""
	}
	cur := c.a.grantByID(g.ID)
	if cur == nil || !cur.Live(time.Now()) {
		return false, "expired"
	}
	if cur.AfkOnly && !c.a.Afk() {
		return false, "not-afk"
	}
	if !grantCovers(cur, b) {
		return false, deck.ReasonDenied
	}
	if cur.PerMinute > 0 {
		c.mu.Lock()
		now := time.Now()
		kept := c.rate[:0]
		for _, t := range c.rate {
			if now.Sub(t) < time.Minute {
				kept = append(kept, t)
			}
		}
		c.rate = kept
		over := len(c.rate) >= cur.PerMinute
		if !over {
			c.rate = append(c.rate, now)
		}
		c.mu.Unlock()
		if over {
			return false, deck.ReasonRateLimited
		}
	}
	return true, ""
}

func grantCovers(g *config.Grant, b *layout.Button) bool {
	if g.All {
		return true
	}
	for _, id := range g.Buttons {
		if id == b.ID {
			return true
		}
	}
	for _, p := range g.Panels {
		if p == b.Panel {
			return true
		}
	}
	return false
}

func (c *client) handle(m msg) {
	a := c.a
	switch m.T {
	case "hb":
		return
	case "hello-native":
		// The Android app says what it is once after connecting; kept for the devices list.
		if c.p.grant == nil && len(m.Text) <= 4096 {
			var info any
			if json.Unmarshal([]byte(m.Text), &info) != nil {
				info = m.Text
			}
			c.mu.Lock()
			c.nativeInfo = info
			c.mu.Unlock()
			a.Bus.Publish("clients", a.clientSummary())
		}
		return
	case "ping":
		c.send(map[string]any{"t": "pong", "n": m.N})
		return
	case "tap", "down", "up":
		c.press(m)
		return
	case "request":
		a.addRequest(c, m)
		return
	case "answered":
		// A linked deck answered a request we sent it.
		if c.peer {
			what := m.Label
			if what == "" {
				what = m.Text
			}
			a.broadcastOwners(map[string]any{"t": "toast", "text": c.name() + ": " + what + " " + m.Result})
		}
		return
	}
	if c.p.grant != nil {
		return // the rest is for the owner's decks
	}
	switch m.T {
	case "voice":
		if m.Mode == "tap" && m.On {
			a.voice.tap(c)
		} else if m.Mode == "tap" {
			a.voice.untap(c)
		} else {
			a.voice.listen(c, m.On)
		}
	case "voice-live":
		_ = a.Store.Update(func(cfg *config.Config) { cfg.AlwaysListen = m.On })
		a.broadcastOwners(map[string]any{"t": "voice-live", "on": m.On})
	case "voice-route":
		if m.Mode == "command" || m.Mode == "auto" || m.Mode == "agent" {
			_ = a.Store.Update(func(cfg *config.Config) { cfg.VoiceRoute = m.Mode })
			a.broadcastOwners(map[string]any{"t": "agent-info", "agent": a.agentInfo()})
		}
	case "chat":
		a.AskAgent(m.Text, "deck")
	case "chat-clear":
		a.ClearChat()
	case "open":
		a.openLink(m.Index, c)
	case "reset-states":
		a.Engine.ResetStates()
	case "release-all":
		a.Engine.ReleaseAll(deck.ReasonReleaseAll)
		c.send(map[string]any{"t": "toast", "text": "All keys released"})
	case "afk":
		a.SetAfk(m.On)
	case "answer":
		a.answerRequest(c, m.ID, m.Do)
	case "share":
		a.shareFromDeck(c, m)
	case "view":
		if a.inCockpit(c) {
			// A console's first ask just gets its share; a pick changes the whole cockpit.
			if m.Mode == "init" || !a.SetCockpitView(m.ID) {
				a.pushCockpit()
			}
			return
		}
		if v, ok := a.ViewByID(m.ID); ok {
			a.sendSolo(c, v)
		}
	case "cockpit-move": // a panel dragged to another console
		if err := a.MovePanel(m.ID, m.Text, c.consoleKey()); err != nil {
			c.send(map[string]any{"t": "toast", "text": err.Error()})
		}
	case "cockpit-add": // a panel added on this screen, for the view it shows
		if err := a.AddToConsole(c, m.ID); err != nil {
			c.send(map[string]any{"t": "toast", "text": err.Error()})
		}
	case "cockpit-remove": // a panel taken off this screen
		if err := a.RemoveFromConsole(c, m.ID); err != nil {
			c.send(map[string]any{"t": "toast", "text": err.Error()})
		}
	case "cockpit-fallback": // where a lost console's panels go
		a.PickFallback(m.ID, m.Text)
	case "identify-ask": // show a console's name on it, to tell the screens apart
		a.Identify(m.ID)
	case "cockpit-solo": // this deck leaves (or rejoins) the cockpit
		solo := m.On
		_ = a.UpdateConsole(c.consoleKey(), nil, nil, &solo)
	case "theme":
		if a.inCockpit(c) && m.ID != "" {
			a.SetCockpitTheme(m.ID)
		}
	case "save-view":
		var v config.View
		if json.Unmarshal([]byte(m.Text), &v) == nil {
			if saved, err := a.SaveView(v); err != nil {
				c.send(map[string]any{"t": "toast", "text": err.Error()})
			} else {
				c.send(map[string]any{"t": "toast", "text": "Saved " + saved.Name})
			}
		}
	case "delete-view":
		a.DeleteView(m.ID)
	case "crew-press":
		a.crew.press(m.Peer, m.ID, c)
	case "crew-request":
		a.crew.request(m.Peer, m.Text, m.ID, c)
	}
}

func (c *client) press(m msg) {
	a := c.a
	b := c.layout.Buttons[m.ID]
	if b == nil && c.p.grant == nil {
		// Views press catalog controls by their catalog id.
		if _, byID := a.Catalog(); byID[m.ID] != nil {
			b = byID[m.ID].Button
		}
	}
	if b == nil {
		return
	}
	if m.T == "up" {
		c.mu.Lock()
		h := c.holds[b.ID]
		delete(c.holds, b.ID)
		c.mu.Unlock()
		a.Engine.Up(h)
		return
	}
	if ok, why := c.allowed(b); !ok {
		c.send(map[string]any{"t": "blocked", "id": b.ID, "reason": why})
		return
	}
	if m.T == "down" && c.p.grant != nil && c.p.grant.NoHold {
		m.T = "tap"
	}
	var r deck.Result
	if m.T == "tap" {
		kind := "tap"
		if b.Mode == "timed" {
			kind = "timed"
		}
		r = a.Engine.Tap(b, c.source(), 0, kind)
	} else {
		c.mu.Lock()
		_, held := c.holds[b.ID]
		c.mu.Unlock()
		if held {
			return
		}
		var h *deck.Hold
		h, r = a.Engine.Down(b, c.source(), func(reason string) {
			c.mu.Lock()
			delete(c.holds, b.ID)
			c.mu.Unlock()
			c.send(map[string]any{"t": "released", "id": b.ID, "reason": reason})
		})
		if h != nil {
			c.mu.Lock()
			c.holds[b.ID] = h
			c.mu.Unlock()
		}
	}
	if !r.OK {
		c.send(map[string]any{"t": "blocked", "id": b.ID, "reason": r.Reason})
		return
	}
	if c.p.grant != nil {
		a.countGrantPress(c.p.grant.ID)
		// The owner sees what guests press.
		a.broadcastOwners(map[string]any{"t": "toast", "text": c.name() + ": " + b.Label})
	}
}

// helloFor is the first message a deck gets: what it may do and the current lights.
func (a *App) helloFor(c *client) map[string]any {
	cfg := a.Store.Config()
	h := map[string]any{"t": "hello", "name": cfg.Name, "states": a.Engine.States(), "afk": a.Afk(),
		"game": a.Engine.Driver().Game(), "focusMode": cfg.FocusMode}
	if g := c.p.grant; g != nil {
		allowed := []string{}
		for _, id := range c.layout.Order {
			if grantCovers(g, c.layout.Buttons[id]) {
				allowed = append(allowed, id)
			}
		}
		h["guest"] = map[string]any{"label": g.Label, "expires": g.Expires, "afkOnly": g.AfkOnly, "noHold": g.NoHold,
			"allowed": allowed, "buttons": a.buttonList(c.layout, allowed), "groups": a.GuestGroups(c.layout, g, nil)}
		h["voice"] = false
		h["voiceStatus"] = map[string]any{"state": "off"}
		return h
	}
	h["voice"] = cfg.Voice
	h["voiceStatus"] = a.voice.status()
	h["views"] = a.Views()
	h["grants"] = a.grantList()
	h["groups"] = a.Groups()
	h["chat"] = a.chatLog()
	h["live"] = cfg.AlwaysListen
	h["agent"] = a.agentInfo()
	h["battle"] = a.battleNow()
	if a.inCockpit(c) {
		h["cockpit"] = true
		h["cockpitTheme"] = a.Store.State().Cockpit.Theme
	}
	h["requests"] = a.reqs.open()
	h["crew"] = a.crew.summary()
	return h
}

func (a *App) buttonList(l *layout.Layout, ids []string) []map[string]any {
	out := []map[string]any{}
	for _, id := range ids {
		b := l.Buttons[id]
		out = append(out, map[string]any{"id": b.ID, "label": b.Label, "panel": b.Panel, "mode": b.Mode})
	}
	return out
}

// ---------- broadcasts ----------

func (a *App) each(fn func(c *client)) {
	a.clientsMu.Lock()
	list := make([]*client, 0, len(a.clients))
	for c := range a.clients {
		list = append(list, c)
	}
	a.clientsMu.Unlock()
	for _, c := range list {
		fn(c)
	}
}

func (a *App) broadcast(v any) { a.each(func(c *client) { c.send(v) }) }

func (a *App) broadcastOwners(v any) {
	a.each(func(c *client) {
		if c.p.grant == nil {
			c.send(v)
		}
	})
}

func (a *App) clientSummary() []map[string]any {
	out := []map[string]any{}
	a.each(func(c *client) {
		kind := "owner"
		if c.peer {
			kind = "crew"
		} else if c.p.grant != nil {
			kind = "guest"
		}
		e := map[string]any{"id": c.id, "name": c.name(), "kind": kind, "local": c.p.local, "layout": c.layout.Name}
		if kind == "owner" {
			c.mu.Lock()
			e["device"], e["console"], e["platform"] = c.p.deviceID, c.consoleKey(), c.platform
			if c.nativeApp != "" {
				e["native"] = map[string]any{"app": c.nativeApp, "info": c.nativeInfo}
			}
			c.mu.Unlock()
		}
		out = append(out, e)
	})
	return out
}

// forwardEvents passes lights and voice results from the engine to every deck.
func (a *App) forwardEvents(ctx context.Context) {
	ch, done := a.Bus.Subscribe()
	defer done()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			switch ev.T {
			case "state":
				d := ev.Data.(map[string]any)
				a.broadcast(map[string]any{"t": "state", "group": d["group"], "on": d["on"]})
			case "states":
				a.broadcast(map[string]any{"t": "states", "states": map[string]bool{}})
			case "grants":
				a.broadcastOwners(map[string]any{"t": "grants", "grants": a.grantList()})
			case "agent":
				d := ev.Data.(map[string]any)
				a.broadcastOwners(map[string]any{"t": "agent", "tool": d["tool"], "ok": d["ok"]})
			case "press":
				d := ev.Data.(map[string]any)
				if ok, _ := d["ok"].(bool); ok {
					src, _ := d["src"].(deck.Source)
					a.broadcastOwners(map[string]any{"t": "pressed", "label": d["label"], "keys": d["keys"], "who": src.Who, "kind": src.Kind})
				}
			}
		}
	}
}
