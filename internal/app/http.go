package app

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tabletdeck "github.com/xattribution/tablet-deck-spooned"
	"github.com/xattribution/tablet-deck-spooned/internal/config"
	"github.com/xattribution/tablet-deck-spooned/internal/deck"
	"github.com/xattribution/tablet-deck-spooned/internal/keys"
	"github.com/xattribution/tablet-deck-spooned/internal/layout"
)

func sourceVoice() deck.Source { return deck.Source{Kind: "voice", Who: "Voice", Local: true} }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// guestPaths are all the guest port answers: share links and what their deck needs.
func guestPath(p string) bool {
	switch {
	case p == "/gws", p == "/layout.json", p == "/sw.js", p == "/sw-app.js", strings.HasPrefix(p, "/g/"):
		return true
	case strings.HasPrefix(p, "/assets/") && !strings.Contains(p, ".."): // the tablet app's files
		return true
	case p == "/" || strings.Contains(p, "..") || strings.HasPrefix(p, "/editor") || strings.HasPrefix(p, "/control") ||
		strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/showcase") || p == "/mcp":
		return false
	}
	// The deck's own files (scripts, styles, icons).
	ext := path.Ext(p)
	return ext == ".js" || ext == ".css" || ext == ".png" || ext == ".svg" || ext == ".webmanifest"
}

func (a *App) routes(guest bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) { a.handleWS(w, r, guest) })
	mux.HandleFunc("/gws", func(w http.ResponseWriter, r *http.Request) { a.handleWS(w, r, guest) })
	mux.HandleFunc("/layout.json", func(w http.ResponseWriter, r *http.Request) { a.handleLayout(w, r, guest) })
	mux.HandleFunc("/layouts.json", a.owner(guest, a.handleLayouts))
	mux.HandleFunc("/voice.json", a.owner(guest, func(w http.ResponseWriter, r *http.Request, _ principal) {
		st := a.voice.status()
		cmds := a.voice.Commands()
		list := make([][]string, 0, len(cmds))
		for _, c := range cmds {
			list = append(list, []string{c[0], c[1]})
		}
		writeJSON(w, 200, map[string]any{"on": a.Store.Config().Voice, "ready": st["state"] == "ready", "state": st["state"],
			"reason": st["reason"], "commands": list})
	}))
	mux.HandleFunc("/links.json", a.owner(guest, func(w http.ResponseWriter, r *http.Request, _ principal) {
		names := []string{}
		for _, l := range a.Store.Config().Links {
			names = append(names, l.Name)
		}
		writeJSON(w, 200, map[string]any{"links": names})
	}))
	mux.HandleFunc("/events", a.owner(guest, a.handleShowcaseEvents))
	mux.HandleFunc("/editor/", a.owner(guest, a.handleEditor))
	mux.HandleFunc("/api/v1/", a.owner(guest, a.handleAPI))
	mux.HandleFunc("/mcp", a.owner(guest, a.handleMCP))
	mux.HandleFunc("/control", func(w http.ResponseWriter, r *http.Request) {
		if !isLocal(r, guest) {
			http.Error(w, "The control panel only opens on the PC running Tablet Deck.", 403)
			return
		}
		a.serveFile(w, r, "control.html")
	})
	mux.HandleFunc("/g/", func(w http.ResponseWriter, r *http.Request) { a.serveFile(w, r, a.deckPage()) })
	// The old deck stays at /legacy/ while the tablet app takes over /.
	mux.HandleFunc("/legacy/", func(w http.ResponseWriter, r *http.Request) { a.serveFile(w, r, "index.html") })
	mux.HandleFunc("/legacy", func(w http.ResponseWriter, r *http.Request) { a.serveFile(w, r, "index.html") })
	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		name := "app" + r.URL.Path // app/assets/<hashed file>
		if strings.Contains(r.URL.Path, "..") || !fileExists(a.Public, name) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // names change with content
		http.ServeFileFS(w, r, a.Public, name)
	})
	for _, f := range []string{"/app.webmanifest", "/sw-app.js"} { // the tablet app ships its own
		mux.HandleFunc(f, func(w http.ResponseWriter, r *http.Request) {
			name := "app" + r.URL.Path
			if !fileExists(a.Public, name) {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "no-cache")
			if strings.HasSuffix(name, ".js") {
				w.Header().Set("Service-Worker-Allowed", "/")
			}
			http.ServeFileFS(w, r, a.Public, name)
		})
	}
	mux.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		// The service worker may control the whole site, including share links.
		w.Header().Set("Service-Worker-Allowed", "/")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/javascript")
		b, _ := fs.ReadFile(a.Public, "sw.js")
		_, _ = w.Write([]byte(strings.ReplaceAll(string(b), "__VERSION__", tabletdeck.Version)))
	})
	files := http.FileServerFS(a.Public)
	layouts, _ := fs.Sub(tabletdeck.Files, "layouts")
	layoutFiles := http.StripPrefix("/layouts/", http.FileServerFS(layouts))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		if strings.HasPrefix(r.URL.Path, "/layouts/") { // the deck's demo mode
			layoutFiles.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/" && a.hasApp() {
			a.serveFile(w, r, appPage)
			return
		}
		files.ServeHTTP(w, r)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(r.URL.Path, "/control") || strings.HasPrefix(r.URL.Path, "/editor") {
			h.Set("X-Frame-Options", "DENY")
		}
		if guest && !guestPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// appPage is the tablet app (built from web/ into public/app/). Until it is built,
// the old deck in public/index.html is the deck.
const appPage = "app/index.html"

func (a *App) hasApp() bool { return fileExists(a.Public, appPage) }

// deckPage is the page a tablet or share link opens.
func (a *App) deckPage() string {
	if a.hasApp() {
		return appPage
	}
	return "index.html"
}

func fileExists(fsys fs.FS, name string) bool {
	fi, err := fs.Stat(fsys, name)
	return err == nil && !fi.IsDir()
}

func (a *App) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	b, err := fs.ReadFile(a.Public, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	// Share links live at /g/<token>; the deck's files are at the root. (Only the deck:
	// a base on the control panel would send its #section links to the deck.)
	if name == "index.html" || name == appPage {
		b = []byte(strings.Replace(string(b), "<head>", `<head><base href="/">`, 1))
	}
	_, _ = w.Write(b)
}

// owner wraps a handler that needs owner credentials.
func (a *App) owner(guest bool, h func(http.ResponseWriter, *http.Request, principal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, status := a.authOwner(r, guest)
		if status != 0 {
			msg := "Wrong pairing code."
			if status == http.StatusTooManyRequests {
				msg = "Too many wrong pairing codes. Wait a minute."
			}
			jsonErr(w, status, msg)
			return
		}
		h(w, r, p)
	}
}

// handleLayout sends a layout to draw: the owner's choice, or a guest's shared one
// with the buttons it may press.
func (a *App) handleLayout(w http.ResponseWriter, r *http.Request, guest bool) {
	q := r.URL.Query()
	var l *layout.Layout
	var err error
	var allowed []string
	if q.Get("g") != "" {
		p, status := a.authGuest(r)
		if status != 0 {
			jsonErr(w, status, map[int]string{410: "This share link ran out."}[status])
			return
		}
		if l, err = a.LayoutFile(p.grant.Layout); err == nil {
			allowed = []string{}
			for _, id := range l.Order {
				if grantCovers(p.grant, l.Buttons[id]) {
					allowed = append(allowed, id)
				}
			}
		}
	} else {
		if _, status := a.authOwner(r, guest); status != 0 {
			jsonErr(w, status, "Wrong pairing code.")
			return
		}
		l, err = a.Layout(q.Get("device"), q.Get("layout"))
	}
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	if allowed != nil {
		raw := map[string]any{}
		for k, v := range l.Raw {
			raw[k] = v
		}
		raw["allowed"] = allowed
		writeJSON(w, 200, raw)
		return
	}
	writeJSON(w, 200, l.Raw)
}

// handleLayouts lists the layouts a device can pick: those made for its screen (phone
// layouts are under 500 px tall), plus grid layouts, which fit any screen.
func (a *App) handleLayouts(w http.ResponseWriter, r *http.Request, _ principal) {
	phone := r.URL.Query().Get("device") == "phone"
	type item struct {
		File   string `json:"file"`
		Name   string `json:"name"`
		Custom bool   `json:"custom"`
	}
	out := []item{}
	for _, f := range a.LayoutFiles() {
		l, err := a.LayoutFile(f)
		if err != nil || (len(l.Canvas) == 2 && l.Phone() != phone) {
			continue
		}
		out = append(out, item{f, l.Name, l.Custom})
	}
	cfg := a.Store.Config()
	pc := cfg.Layout
	if phone && cfg.PhoneLayout != "" {
		pc = cfg.PhoneLayout
	}
	writeJSON(w, 200, map[string]any{"layouts": out, "pcChoice": pc})
}

// handleShowcaseEvents streams presses for the showcase page (public/showcase.html).
func (a *App) handleShowcaseEvents(w http.ResponseWriter, r *http.Request, _ principal) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	cfg := a.Store.Config()
	write := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", b)
		fl.Flush()
	}
	write(map[string]any{"t": "hello", "time": time.Now().UnixMilli(), "focusCheck": cfg.FocusMode != deck.FocusOff,
		"focusProbe": true, "voice": a.voice.status()["state"] == "ready"})
	ch, done := a.Bus.Subscribe()
	defer done()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		case ev := <-ch:
			if ev.T != "press" {
				continue
			}
			d := ev.Data.(map[string]any)
			ok, _ := d["ok"].(bool)
			write(map[string]any{"t": "press", "time": ev.Time.UnixMilli(), "id": d["id"], "label": d["label"], "keys": d["keys"],
				"kind": d["kind"], "blocked": !ok, "reason": d["reason"]})
		}
	}
}

// ---------- open a website on the PC ----------

var lastOpen time.Time

// openLink opens one of the PC's links by position, so a tablet can only open the
// addresses set up on this PC, never one of its own.
func (a *App) openLink(index int, c *client) {
	links := a.Store.Config().Links
	if index < 0 || index >= len(links) || !strings.HasPrefix(links[index].URL, "https://") {
		return
	}
	if time.Since(lastOpen) < 1500*time.Millisecond {
		return
	}
	lastOpen = time.Now()
	l := links[index]
	if a.Opts.DryRun {
		c.send(map[string]any{"t": "opened", "name": l.Name, "dryRun": true})
		return
	}
	if err := openURL(l.URL); err == nil {
		c.send(map[string]any{"t": "opened", "name": l.Name})
	}
}

// ---------- layout editor ----------

var reLayoutName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// handleEditor serves the layout editor's API. It only answers this PC, so a device on
// the network can't change files even with the pairing code.
func (a *App) handleEditor(w http.ResponseWriter, r *http.Request, p principal) {
	if !p.local {
		jsonErr(w, 403, "The layout editor only works on the PC running Tablet Deck (open it at http://localhost).")
		return
	}
	cfg := a.Store.Config()
	switch r.URL.Path {
	case "/editor/layouts":
		type item struct {
			File   string `json:"file"`
			Name   string `json:"name"`
			Canvas []int  `json:"canvas"`
			Custom bool   `json:"custom"`
			Active any    `json:"active"`
		}
		out := []item{}
		for _, f := range a.LayoutFiles() {
			l, err := a.LayoutFile(f)
			if err != nil {
				continue
			}
			var active any
			if f == cfg.Layout {
				active = "tablet"
			} else if f == cfg.PhoneLayout {
				active = "phone"
			}
			out = append(out, item{f, l.Name, l.Canvas, l.Custom, active})
		}
		writeJSON(w, 200, map[string]any{"layouts": out, "keys": keys.Names(), "overrides": a.Binds().Binds})
	case "/editor/layout":
		f := r.URL.Query().Get("file")
		if !a.hasLayout(f) {
			jsonErr(w, 404, "No such layout.")
			return
		}
		b, err := a.readFile(f)
		if err != nil {
			jsonErr(w, 404, "No such layout.")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	case "/editor/save":
		if r.Method != http.MethodPost {
			jsonErr(w, 405, "POST only")
			return
		}
		var body struct {
			Name   string         `json:"name"`
			Layout map[string]any `json:"layout"`
			Use    bool           `json:"use"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&body); err != nil {
			jsonErr(w, 400, "The editor sent something unreadable.")
			return
		}
		name := strings.ToLower(strings.TrimSpace(body.Name))
		if !reLayoutName.MatchString(name) {
			jsonErr(w, 400, "Use letters, numbers and dashes for the name.")
			return
		}
		file := "layouts/" + name + ".json"
		// Only the player's own layouts (marked "custom") may be overwritten.
		if old, err := a.readFile(file); err == nil {
			var m map[string]any
			if json.Unmarshal(old, &m) != nil || m["custom"] != true {
				jsonErr(w, 400, fmt.Sprintf("%q is a built-in layout. Pick another name.", name))
				return
			}
		}
		if body.Layout == nil {
			jsonErr(w, 400, "That isn't a layout.")
			return
		}
		body.Layout["custom"] = true
		data, _ := json.MarshalIndent(body.Layout, "", "  ")
		l, err := layout.Parse(file, data, a.Binds().Binds)
		if err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
		dst := filepath.Join(a.Store.Dir, "layouts", name+".json")
		_ = os.MkdirAll(filepath.Dir(dst), 0o755)
		if err := os.WriteFile(dst, append(data, '\n'), 0o644); err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		a.layoutsMu.Lock()
		delete(a.layouts, file)
		a.layoutsMu.Unlock()
		a.panelGen.Add(1) // the catalog is built from layouts too
		a.Log.Printf("Saved layout %s.", file)
		kind := "layout"
		if l.Phone() {
			kind = "phoneLayout"
		}
		old := a.Store.Config()
		if body.Use {
			_ = a.Store.Update(func(c *config.Config) {
				if kind == "layout" {
					c.Layout = file
				} else {
					c.PhoneLayout = file
				}
			})
		}
		cfg = a.Store.Config()
		if cfg.Layout == file || cfg.PhoneLayout == file {
			a.broadcastOwners(map[string]any{"t": "reload"})
			if kind == "layout" {
				a.voice.reloadPhrases()
			}
		}
		a.ApplyConfig(old, cfg)
		active := (kind == "layout" && cfg.Layout == file) || (kind == "phoneLayout" && cfg.PhoneLayout == file)
		writeJSON(w, 200, map[string]any{"ok": true, "file": file, "active": active})
	default:
		jsonErr(w, 404, "Not found.")
	}
}
