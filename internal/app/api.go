package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	tabletdeck "github.com/xattribution/tablet-deck-spooned"
	"github.com/xattribution/tablet-deck-spooned/internal/audio"
	"github.com/xattribution/tablet-deck-spooned/internal/config"
	"github.com/xattribution/tablet-deck-spooned/internal/deck"
	"github.com/xattribution/tablet-deck-spooned/internal/keys"
	"github.com/xattribution/tablet-deck-spooned/internal/layout"
	"github.com/xattribution/tablet-deck-spooned/internal/panels"
	"github.com/xattribution/tablet-deck-spooned/internal/sc"
)

// apiHolds are buttons held down through the API (a Stream Deck key held), by button id.
var (
	apiHoldsMu sync.Mutex
	apiHolds   = map[string]*deck.Hold{}
)

// handleAPI is the local API for the control panel, Stream Deck and other tools.
// Everything needs owner credentials; settings, keybind files and tunnels also need
// the request to come from this PC.
func (a *App) handleAPI(w http.ResponseWriter, r *http.Request, p principal) {
	route := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	parts := strings.Split(route, "/")
	method := r.Method
	localOnly := func() bool {
		if !p.local {
			jsonErr(w, 403, "Only from the PC running Tablet Deck.")
			return false
		}
		return true
	}
	decode := func(v any) bool {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
			jsonErr(w, 400, "Bad JSON: "+err.Error())
			return false
		}
		return true
	}

	switch {
	case route == "status":
		writeJSON(w, 200, a.status(p.local))

	case route == "buttons":
		l, err := a.apiLayout(r.URL.Query().Get("layout"))
		if err != nil {
			jsonErr(w, 404, err.Error())
			return
		}
		states := a.Engine.States()
		out := []map[string]any{}
		for _, id := range l.Order {
			b := l.Buttons[id]
			e := map[string]any{"id": b.ID, "label": b.Label, "keys": b.Keys, "mode": b.Mode, "panel": b.Panel}
			if b.State != nil {
				e["group"] = b.State.Group
				e["on"] = states[b.State.Group]
			}
			out = append(out, e)
		}
		writeJSON(w, 200, map[string]any{"layout": l.File, "name": l.Name, "panels": l.Panels, "buttons": out})

	case route == "press" && method == http.MethodPost:
		var body struct {
			ID     string `json:"id"`
			Label  string `json:"label"`
			Action string `json:"action"` // a Star Citizen action name, e.g. v_toggle_landing_system
			Keys   string `json:"keys"`   // a raw combo, this PC only
			Mode   string `json:"mode"`   // tap (default), down, up
			Ms     int    `json:"ms"`
			Layout string `json:"layout"`
			Who    string `json:"who"`
		}
		if !decode(&body) {
			return
		}
		res, status := a.apiPress(p, body.ID, body.Label, body.Action, body.Keys, body.Mode, body.Ms, body.Layout, body.Who)
		writeJSON(w, status, res)

	case route == "catalog":
		all, _ := a.Catalog()
		states := a.Engine.States()
		list := make([]map[string]any, 0, len(all))
		for _, c := range all {
			list = append(list, controlView(c, states))
		}
		writeJSON(w, 200, map[string]any{"groups": a.Groups(), "controls": list, "views": a.Views(), "game": a.GameActions().Version})

	// ---------- panels ----------
	case route == "panels" && method == http.MethodGet:
		writeJSON(w, 200, a.Panels())
	case route == "panels" && method == http.MethodPost:
		var pn panels.Panel
		if !decode(&pn) {
			return
		}
		saved, err := a.SavePanel(pn)
		if err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, saved)
	case len(parts) == 2 && parts[0] == "panels" && method == http.MethodGet:
		for _, pn := range a.Panels() {
			if pn.ID == parts[1] {
				writeJSON(w, 200, pn)
				return
			}
		}
		jsonErr(w, 404, "No such panel.")
	case len(parts) == 2 && parts[0] == "panels" && method == http.MethodPut:
		var pn panels.Panel
		if !decode(&pn) {
			return
		}
		pn.ID = parts[1]
		saved, err := a.SavePanel(pn)
		if err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, saved)
	case len(parts) == 2 && parts[0] == "panels" && method == http.MethodDelete:
		writeJSON(w, 200, map[string]any{"ok": a.DeletePanel(parts[1])})
	case route == "panels/guide":
		b, _ := tabletdeck.Files.ReadFile("docs/PANELS.md")
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write(b)
	case route == "cockpit" && method == http.MethodGet:
		writeJSON(w, 200, a.CockpitStatus())
	case route == "cockpit" && method == http.MethodPost:
		var body struct {
			Action, View, Group, To, ID, Theme string
			Name                               *string
			Order                              *int
			Solo                               *bool
		}
		if !decode(&body) {
			return
		}
		var err error
		switch body.Action {
		case "view":
			if !a.SetCockpitView(body.View) {
				err = fmt.Errorf("no view %q", body.View)
			}
		case "move":
			err = a.MovePanel(body.Group, body.To, body.ID)
		case "console":
			err = a.UpdateConsole(body.ID, body.Name, body.Order, body.Solo)
		case "fallback":
			a.PickFallback(body.ID, body.To)
		case "theme":
			a.SetCockpitTheme(body.Theme)
		case "reset":
			a.ResetCockpit()
		case "identify":
			a.Identify(body.ID)
		case "forget":
			if a.consoleOnline(body.ID) {
				err = fmt.Errorf("that console is connected")
			} else {
				a.ForgetConsole(body.ID)
			}
		default:
			err = fmt.Errorf("action: view, move, console, fallback, theme, reset or identify")
		}
		if err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, a.CockpitStatus())
	case route == "agent/ask" && method == http.MethodPost:
		if !localOnly() {
			return
		}
		var body struct{ Text string }
		if !decode(&body) {
			return
		}
		if !a.agentReady() {
			jsonErr(w, 400, "set the agent's API and model first")
			return
		}
		a.AskAgent(body.Text, "api")
		writeJSON(w, 200, map[string]any{"ok": true})
	case route == "export" && method == http.MethodGet:
		writeJSON(w, 200, a.Export(r.URL.Query().Get("view")))
	case route == "import" && method == http.MethodPost:
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
		if err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
		res, err := a.Import(body)
		if err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, res)

	case route == "show-view" && method == http.MethodPost:
		var body struct{ View, Device string }
		if !decode(&body) {
			return
		}
		v, ok := a.ViewByID(body.View)
		if !ok {
			jsonErr(w, 404, "No such view.")
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "decks": a.ShowView(v, body.Device)})

	case route == "views" && method == http.MethodGet:
		writeJSON(w, 200, a.Views())
	case len(parts) == 2 && parts[0] == "views" && method == http.MethodGet:
		v, ok := a.ViewByID(parts[1])
		if !ok {
			jsonErr(w, 404, "No such view.")
			return
		}
		writeJSON(w, 200, map[string]any{"view": v, "groups": a.Resolve(v)})

	case route == "release-all" && method == http.MethodPost:
		a.Engine.ReleaseAll(deck.ReasonReleaseAll)
		writeJSON(w, 200, map[string]any{"ok": true})

	case route == "states" && method == http.MethodGet:
		writeJSON(w, 200, a.Engine.States())
	case route == "states" && method == http.MethodPost:
		var body struct {
			Group string `json:"group"`
			On    bool   `json:"on"`
		}
		if decode(&body) && body.Group != "" {
			a.Engine.SetState(body.Group, body.On)
			writeJSON(w, 200, map[string]any{"ok": true})
		}
	case route == "states/reset" && method == http.MethodPost:
		a.Engine.ResetStates()
		writeJSON(w, 200, map[string]any{"ok": true})

	case route == "events":
		a.streamEvents(w, r)

	case route == "afk" && method == http.MethodPost:
		var body struct {
			On bool `json:"on"`
		}
		if decode(&body) {
			a.SetAfk(body.On)
			writeJSON(w, 200, map[string]any{"ok": true, "afk": a.Afk()})
		}

	case route == "grants" && method == http.MethodGet:
		writeJSON(w, 200, a.grantList())
	case route == "grants" && method == http.MethodPost:
		var spec GrantSpec
		if !decode(&spec) {
			return
		}
		g, err := a.CreateGrant(spec)
		if err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, a.grantView(g))
	case len(parts) == 2 && parts[0] == "grants" && method == http.MethodDelete:
		writeJSON(w, 200, map[string]any{"ok": a.RevokeGrant(parts[1])})

	case route == "devices" && method == http.MethodGet:
		devs := a.Store.State().Devices
		if devs == nil {
			devs = []config.Device{}
		}
		writeJSON(w, 200, devs)
	case len(parts) == 2 && parts[0] == "devices" && method == http.MethodDelete:
		a.unpair(parts[1])
		writeJSON(w, 200, map[string]any{"ok": true})

	case route == "crew" && method == http.MethodGet:
		writeJSON(w, 200, map[string]any{"links": a.Store.State().Crew, "crew": a.crew.summary()})
	case route == "crew" && method == http.MethodPost:
		var body struct{ Name, Link string }
		if !decode(&body) {
			return
		}
		l, err := a.crew.Add(body.Name, body.Link)
		if err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, l)
	case len(parts) == 2 && parts[0] == "crew" && method == http.MethodDelete:
		a.crew.Remove(parts[1])
		writeJSON(w, 200, map[string]any{"ok": true})

	case route == "nearby":
		writeJSON(w, 200, a.finder.List())

	case route == "requests":
		writeJSON(w, 200, a.reqs.open())

	case route == "log":
		writeJSON(w, 200, a.Bus.History())

	case route == "qr.png":
		if !localOnly() {
			return
		}
		text := r.URL.Query().Get("text")
		if text == "" || len(text) > 1000 {
			jsonErr(w, 400, "text is required")
			return
		}
		png, err := qrcode.Encode(text, qrcode.Medium, 512)
		if err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(png)

	case route == "config" && method == http.MethodGet:
		if localOnly() {
			writeJSON(w, 200, a.Store.Config())
		}
	case route == "config" && method == http.MethodPut:
		if !localOnly() {
			return
		}
		old := a.Store.Config()
		next := old
		if !decode(&next) {
			return
		}
		if err := next.Validate(); err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
		if err := a.Store.Update(func(c *config.Config) { *c = next }); err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		a.ApplyConfig(old, next)
		restart := old.Port != next.Port || old.GuestPort != next.GuestPort
		writeJSON(w, 200, map[string]any{"ok": true, "restart": restart})

	case route == "keybinds" && method == http.MethodGet:
		b := a.Binds()
		writeJSON(w, 200, map[string]any{"files": b.Files, "actions": len(b.Binds), "installs": b.Installs})
	case route == "keybinds/unbound" && method == http.MethodGet:
		all, _ := a.Catalog()
		var list []map[string]any
		for _, c := range all {
			if c.Unbound {
				list = append(list, map[string]any{"id": c.ID, "label": c.Label, "group": c.Group, "action": c.ScAction})
			}
		}
		writeJSON(w, 200, map[string]any{"count": len(list), "controls": list})
	case route == "keybinds/bind-all" && method == http.MethodPost:
		if !localOnly() {
			return
		}
		res, err := a.BindAll()
		if err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, res)
	case route == "keybinds/install" && method == http.MethodPost:
		if !localOnly() {
			return
		}
		data, err := a.readFile("keybinds/TabletDeck-StarCitizen.xml")
		if err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		installs := a.Binds().Installs
		if len(installs) == 0 {
			installs = sc.Discover(a.Engine.Driver().Game().Exe, a.Store.Config().GameDirs)
		}
		if len(installs) == 0 {
			jsonErr(w, 404, "Couldn't find Star Citizen. Add its folder under Keybinds > Game folders.")
			return
		}
		written, err := sc.InstallProfile(data, "TabletDeck-StarCitizen.xml", installs)
		if err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "written": written})

	case route == "voice" && method == http.MethodGet:
		if !localOnly() {
			return
		}
		m := a.FindSpeechModels()
		out := map[string]any{"status": a.voice.status(), "models": m, "voices": m.VoiceNames(),
			"mics": audio.Inputs(), "speakers": audio.Outputs(), "commands": len(a.voice.Commands())}
		if n := a.voice.Native(); n != nil {
			out["engine"] = "native"
			out["recognizer"] = n.rec.Name()
			out["level"] = n.lis.Level()
		} else {
			out["engine"] = map[bool]string{true: "windows", false: ""}[a.voice.status()["state"] == "ready"]
		}
		writeJSON(w, 200, out)
	case route == "voice/say" && method == http.MethodPost:
		if !localOnly() {
			return
		}
		var body struct{ Text, Voice string }
		if !decode(&body) {
			return
		}
		n := a.voice.Native()
		if n == nil || n.sp == nil {
			jsonErr(w, 409, "Spoken replies need the native voice engine and a Pocket TTS model.")
			return
		}
		if body.Voice != "" {
			_ = n.sp.SetVoice(body.Voice)
		}
		n.announce(body.Text)
		writeJSON(w, 200, map[string]any{"ok": true})

	case route == "tunnel" && method == http.MethodPost:
		if !localOnly() {
			return
		}
		var body struct {
			On bool `json:"on"`
		}
		if !decode(&body) {
			return
		}
		old := a.Store.Config()
		_ = a.Store.Update(func(c *config.Config) {
			c.Tunnel = map[bool]string{true: "cloudflare", false: ""}[body.On]
		})
		a.ApplyConfig(old, a.Store.Config())
		writeJSON(w, 200, a.tunnel.Status())
	case route == "tunnel/install" && method == http.MethodPost:
		if localOnly() {
			if err := a.installCloudflared(); err != nil {
				jsonErr(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, a.tunnel.Status())
		}

	default:
		jsonErr(w, 404, "No such API route.")
	}
}

func (a *App) apiLayout(file string) (*layout.Layout, error) {
	if file != "" {
		if !a.hasLayout(file) {
			return nil, fmt.Errorf("no layout %s", file)
		}
		return a.LayoutFile(file)
	}
	return a.Layout("tablet", "")
}

// apiPress presses a button found by id, label or Star Citizen action, or (from this PC)
// a raw key combo.
func (a *App) apiPress(p principal, id, label, action, combo, mode string, ms int, file, who string) (map[string]any, int) {
	var b *layout.Button
	if combo != "" {
		if !p.local {
			return map[string]any{"ok": false, "error": "raw keys only work from this PC"}, 403
		}
		scans, err := keys.ParseCombo(combo)
		if err != nil {
			return map[string]any{"ok": false, "error": err.Error()}, 400
		}
		b = &layout.Button{ID: "keys:" + combo, Label: combo, Keys: combo, Scans: scans}
	} else {
		// Catalog ids first (landing-gear), then the layout's own ids and labels.
		for _, ref := range []string{id, label, action} {
			if ref == "" || file != "" {
				continue
			}
			if c := a.FindControl(ref); c != nil {
				b = c.Button
				break
			}
		}
		l, err := a.apiLayout(file)
		if err != nil {
			return map[string]any{"ok": false, "error": err.Error()}, 404
		}
		if b == nil {
			b = findButton(l, id, label, action)
		}
		if b == nil && action != "" {
			// Not on the deck, but the player has it bound in the game.
			if c, ok := a.Binds().Binds[action]; ok {
				if scans, err := keys.ParseCombo(c); err == nil {
					b = &layout.Button{ID: "action:" + action, Label: action, Keys: c, ScAction: action, Scans: scans}
				}
			}
		}
		if b == nil {
			return map[string]any{"ok": false, "error": "no such button"}, 404
		}
	}
	if who == "" {
		who = "API"
	}
	src := deck.Source{Kind: "api", Who: who, Local: p.local}
	var r deck.Result
	switch mode {
	case "down":
		apiHoldsMu.Lock()
		_, held := apiHolds[b.ID]
		apiHoldsMu.Unlock()
		if held {
			return map[string]any{"ok": true, "held": true}, 200
		}
		var h *deck.Hold
		h, r = a.Engine.Down(b, src, func(string) {
			apiHoldsMu.Lock()
			if apiHolds[b.ID] == h {
				delete(apiHolds, b.ID)
			}
			apiHoldsMu.Unlock()
		})
		if h != nil {
			apiHoldsMu.Lock()
			apiHolds[b.ID] = h
			apiHoldsMu.Unlock()
		}
	case "up":
		apiHoldsMu.Lock()
		h := apiHolds[b.ID]
		delete(apiHolds, b.ID)
		apiHoldsMu.Unlock()
		a.Engine.Up(h)
		return map[string]any{"ok": true}, 200
	default:
		r = a.Engine.Tap(b, src, ms, "api")
	}
	out := map[string]any{"ok": r.OK, "id": b.ID, "label": b.Label, "keys": b.Keys}
	if !r.OK {
		out["reason"] = r.Reason
	}
	return out, 200
}

func findButton(l *layout.Layout, id, label, action string) *layout.Button {
	if b := l.Buttons[id]; id != "" && b != nil {
		return b
	}
	for _, bid := range l.Order {
		b := l.Buttons[bid]
		if (label != "" && strings.EqualFold(b.Label, label)) || (action != "" && b.ScAction == action) {
			return b
		}
	}
	return nil
}

func (a *App) unpair(id string) {
	_ = a.Store.UpdateState(func(s *config.State) {
		kept := s.Devices[:0]
		for _, d := range s.Devices {
			if d.ID != id {
				kept = append(kept, d)
			}
		}
		s.Devices = kept
	})
	a.ForgetConsole(id)
	a.each(func(c *client) {
		if c.p.deviceID == id {
			c.kick(map[string]any{"t": "unpaired"})
		}
	})
	a.Bus.Publish("devices", nil)
}

func (a *App) grantList() []map[string]any {
	out := []map[string]any{}
	for _, g := range a.Store.State().Grants {
		if g.Live(time.Now()) {
			out = append(out, a.grantView(g))
		}
	}
	return out
}

func (a *App) grantView(g config.Grant) map[string]any {
	connected := 0
	a.each(func(c *client) {
		if c.p.grant != nil && c.p.grant.ID == g.ID {
			connected++
		}
	})
	return map[string]any{"id": g.ID, "label": g.Label, "kind": g.Kind, "layout": g.Layout, "all": g.All, "panels": g.Panels,
		"buttons": g.Buttons, "afkOnly": g.AfkOnly, "noHold": g.NoHold, "perMinute": g.PerMinute, "created": g.Created,
		"expires": g.Expires, "live": g.Live(time.Now()), "presses": g.Presses, "connected": connected, "links": a.GrantLinks(g)}
}

// status is everything the control panel's dashboard shows.
func (a *App) status(local bool) map[string]any {
	cfg := a.Store.Config()
	presses, blocked := a.Engine.Counts()
	b := a.Binds()
	out := map[string]any{
		"version": tabletdeck.Version, "name": cfg.Name, "port": cfg.Port, "dryRun": a.Opts.DryRun,
		"driver": a.Engine.Driver().Name(), "game": a.Engine.Driver().Game(), "focusMode": cfg.FocusMode,
		"presses": presses, "blocked": blocked, "clients": a.clientSummary(), "voice": a.voice.status(),
		"tunnel": a.tunnel.Status(), "afk": a.Afk(), "uptime": int(time.Since(a.started).Seconds()),
		"keybinds": map[string]any{"files": b.Files, "actions": len(b.Binds), "installs": b.Installs},
		"devices":  len(a.Store.State().Devices), "requests": a.reqs.open(),
	}
	if local {
		lan := a.LanAddress()
		out["pairing"] = map[string]any{"code": cfg.Token, "link": fmt.Sprintf("http://%s:%d/?t=%s", lan, cfg.Port, cfg.Token),
			"addresses": Addresses(), "local": fmt.Sprintf("http://localhost:%d/?t=%s&device=tablet", cfg.Port, cfg.Token)}
		out["apiKey"] = cfg.APIKey
		out["configPath"] = a.Store.ConfigPath()
		out["editor"] = fmt.Sprintf("http://localhost:%d/editor.html?t=%s", cfg.Port, cfg.Token)
	}
	return out
}

// streamEvents is the API's live event stream (Server-Sent Events), for the control
// panel and tools like the Stream Deck plugin that light keys from button states.
func (a *App) streamEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	ch, done := a.Bus.Subscribe()
	defer done()
	b, _ := json.Marshal(map[string]any{"t": "states", "data": map[string]any{"states": a.Engine.States()}})
	fmt.Fprintf(w, "data: %s\n\n", b)
	fl.Flush()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
		case ev := <-ch:
			b, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		fl.Flush()
	}
}

// installCloudflared downloads Cloudflare's tunnel client into the data folder, when
// the player clicks to set up internet share links.
func (a *App) installCloudflared() error {
	const u = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe"
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 200<<20))
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(data, []byte("MZ")) {
		return fmt.Errorf("the download isn't a Windows program")
	}
	dst := filepath.Join(a.Store.Dir, "cloudflared.exe")
	if err := os.WriteFile(dst+".tmp", data, 0o755); err != nil {
		return err
	}
	return os.Rename(dst+".tmp", dst)
}
