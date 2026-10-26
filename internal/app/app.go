// Package app is the Tablet Deck server: it loads layouts and keybinds, runs the key
// engine, serves the deck to tablets and phones, and links decks into a crew.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tabletdeck "github.com/xattribution/tablet-deck-spooned"
	"github.com/xattribution/tablet-deck-spooned/internal/config"
	"github.com/xattribution/tablet-deck-spooned/internal/deck"
	"github.com/xattribution/tablet-deck-spooned/internal/input"
	"github.com/xattribution/tablet-deck-spooned/internal/layout"
	"github.com/xattribution/tablet-deck-spooned/internal/sc"
)

// Options are set on the command line.
type Options struct {
	DryRun  bool
	DataDir string
	Port    int // overrides config.json when set
	Driver  input.Driver
	Log     *log.Logger
}

// App is a running Tablet Deck.
type App struct {
	Opts   Options
	Store  *config.Store
	Engine *deck.Engine
	Bus    *deck.Bus
	Log    *log.Logger
	Public fs.FS // the deck web app

	binds     atomic.Pointer[bindSet]
	layoutsMu sync.Mutex
	layouts   map[string]*layout.Layout // by file, for the current binds

	clientsMu sync.Mutex
	clients   map[*client]struct{}

	pair    *limiter
	afk     atomic.Bool
	reqs    *requests
	voice   *voice
	crew    *crew
	finder  *finder
	tunnel  *tunnel
	started time.Time

	srv, guestSrv *http.Server
}

// bindSet is the keybinds in use and where they came from.
type bindSet struct {
	Binds    map[string]string
	Files    []string
	Installs []sc.Install
	stamp    string // changes when any file changes
}

// New loads everything; Start serves it.
func New(opts Options) (*App, error) {
	if opts.Log == nil {
		opts.Log = log.New(os.Stdout, "", log.LstdFlags)
	}
	if opts.DataDir == "" {
		opts.DataDir = config.DataDir()
	}
	store, err := config.Open(opts.DataDir)
	if err != nil {
		return nil, err
	}
	if opts.Port != 0 {
		cfg := store.Config()
		if cfg.Port != opts.Port {
			_ = store.Update(func(c *config.Config) { c.Port, c.GuestPort = opts.Port, opts.Port+1 })
		}
	}
	pub, err := fs.Sub(tabletdeck.Files, "public")
	if err != nil {
		return nil, err
	}
	drv := opts.Driver
	if drv == nil {
		drv = input.New(opts.DryRun)
	}
	cfg := store.Config()
	drv.SetGames(cfg.GameProcesses)
	bus := deck.NewBus(300)
	a := &App{Opts: opts, Store: store, Bus: bus, Log: opts.Log, Public: pub,
		layouts: map[string]*layout.Layout{}, clients: map[*client]struct{}{}, pair: newLimiter(),
		reqs: newRequests(), started: time.Now()}
	a.Engine = deck.New(drv, engineSettings(cfg), bus)
	a.refreshBinds(true)
	if _, err := a.Layout("tablet", ""); err != nil {
		return nil, err
	}
	store.PruneGrants()
	return a, nil
}

func engineSettings(c config.Config) deck.Settings {
	return deck.Settings{TapMs: c.TapMs, ComboGapMs: c.ComboGapMs, FocusMode: c.FocusMode, SettleMs: c.SettleMs,
		ReleaseOnFocusLoss: c.ReleaseOnFocusLoss, MaxHoldMs: c.MaxHoldMs}
}

// ---------- files ----------

// readFile reads a layout or keybind file: the player's own copy in the data folder
// first, then the bundled one. Absolute paths are used as they are.
func (a *App) readFile(name string) ([]byte, error) {
	if filepath.IsAbs(name) {
		return os.ReadFile(name)
	}
	clean := filepath.ToSlash(filepath.Clean(name))
	if strings.HasPrefix(clean, "../") {
		return nil, fs.ErrNotExist
	}
	if b, err := os.ReadFile(filepath.Join(a.Store.Dir, filepath.FromSlash(clean))); err == nil {
		return b, nil
	}
	return fs.ReadFile(tabletdeck.Files, clean)
}

// LayoutFiles lists every layout: bundled ones plus the player's own.
func (a *App) LayoutFiles() []string {
	set := map[string]bool{}
	if ents, err := fs.ReadDir(tabletdeck.Files, "layouts"); err == nil {
		for _, e := range ents {
			if strings.HasSuffix(e.Name(), ".json") {
				set["layouts/"+e.Name()] = true
			}
		}
	}
	if ents, err := os.ReadDir(filepath.Join(a.Store.Dir, "layouts")); err == nil {
		for _, e := range ents {
			if strings.HasSuffix(e.Name(), ".json") {
				set["layouts/"+e.Name()] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for f := range set {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func (a *App) hasLayout(file string) bool {
	for _, f := range a.LayoutFiles() {
		if f == file {
			return true
		}
	}
	return false
}

// ---------- keybinds ----------

// refreshBinds rereads the keybind files when any of them changed (or the game started
// from another channel), and tells the decks to reload. Polled, so rebinding in-game
// shows up on the deck within a couple of seconds.
func (a *App) refreshBinds(force bool) {
	cfg := a.Store.Config()
	g := a.Engine.Driver().Game()
	var installs []sc.Install
	files := append([]string(nil), cfg.Actionmaps...)
	if cfg.FollowGameBinds {
		installs = sc.Discover(g.Exe, cfg.GameDirs)
		// The running channel, else LIVE, else whatever was found first.
		for _, in := range installs {
			if in.Actionmaps != "" {
				files = append(files, in.Actionmaps)
				break
			}
		}
	}
	var stamp strings.Builder
	var resolved []string
	for _, f := range files {
		p := f
		if !filepath.IsAbs(p) {
			if _, err := os.Stat(filepath.Join(a.Store.Dir, p)); err == nil {
				p = filepath.Join(a.Store.Dir, p)
			}
		}
		if fi, err := os.Stat(p); err == nil {
			fmt.Fprintf(&stamp, "%s|%d|%d;", p, fi.Size(), fi.ModTime().UnixNano())
		} else {
			fmt.Fprintf(&stamp, "%s|bundled;", p)
		}
		resolved = append(resolved, p)
	}
	old := a.binds.Load()
	if !force && old != nil && old.stamp == stamp.String() {
		return
	}
	binds := map[string]string{}
	for _, f := range resolved {
		data, err := a.readFile(f)
		if err != nil {
			continue
		}
		for k, v := range sc.ParseBinds(strings.NewReader(string(data))) {
			binds[k] = v
		}
	}
	a.binds.Store(&bindSet{Binds: binds, Files: resolved, Installs: installs, stamp: stamp.String()})
	a.layoutsMu.Lock()
	a.layouts = map[string]*layout.Layout{}
	a.layoutsMu.Unlock()
	if old != nil {
		a.Log.Printf("Keybinds changed (%d actions from %d files); decks reload.", len(binds), len(resolved))
		a.broadcastOwners(map[string]any{"t": "reload"})
		a.Bus.Publish("keybinds", map[string]any{"files": resolved, "actions": len(binds)})
		if a.voice != nil {
			a.voice.restart()
		}
	}
}

// Binds returns the keybinds in use.
func (a *App) Binds() *bindSet { return a.binds.Load() }

// Layout returns a parsed layout: the one asked for when it exists, else the PC's choice
// for the device ("phone" or "tablet").
func (a *App) Layout(device, chosen string) (*layout.Layout, error) {
	cfg := a.Store.Config()
	file := cfg.Layout
	if device == "phone" && cfg.PhoneLayout != "" {
		if _, err := a.readFile(cfg.PhoneLayout); err == nil {
			file = cfg.PhoneLayout
		}
	}
	if chosen != "" && a.hasLayout(chosen) {
		file = chosen
	}
	return a.LayoutFile(file)
}

// LayoutFile parses one layout file (cached until the keybinds change).
func (a *App) LayoutFile(file string) (*layout.Layout, error) {
	a.layoutsMu.Lock()
	defer a.layoutsMu.Unlock()
	if l, ok := a.layouts[file]; ok {
		return l, nil
	}
	data, err := a.readFile(file)
	if err != nil {
		return nil, fmt.Errorf("layout %s not found", file)
	}
	l, err := layout.Parse(file, data, a.binds.Load().Binds)
	if err != nil {
		return nil, err
	}
	a.layouts[file] = l
	return l, nil
}

// ---------- running ----------

// Start listens on the deck port (all interfaces) and the guest port (this PC only, for
// a tunnel), and runs the background jobs. It returns once both are listening.
func (a *App) Start(ctx context.Context) error {
	cfg := a.Store.Config()
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("port %d is taken (is Tablet Deck already running?): %w", cfg.Port, err)
	}
	gln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.GuestPort))
	if err != nil {
		ln.Close()
		return fmt.Errorf("guest port %d is taken: %w", cfg.GuestPort, err)
	}
	a.srv = &http.Server{Handler: a.routes(false), ReadHeaderTimeout: 10 * time.Second}
	a.guestSrv = &http.Server{Handler: a.routes(true), ReadHeaderTimeout: 10 * time.Second}
	go a.serve(a.srv, ln)
	go a.serve(a.guestSrv, gln)

	a.voice = newVoice(a)
	a.voice.start()
	a.crew = newCrew(a)
	a.crew.start(ctx)
	a.finder = newFinder(a)
	a.finder.start(ctx)
	a.tunnel = newTunnel(a)
	if cfg.Tunnel != "" {
		go a.tunnel.Start()
	}
	go a.background(ctx)
	go a.forwardEvents(ctx)
	a.Log.Printf("Tablet Deck %s on port %d (%s)%s", tabletdeck.Version, cfg.Port, a.Engine.Driver().Name(),
		map[bool]string{true: " - DRY RUN, no keys are pressed", false: ""}[a.Opts.DryRun])
	return nil
}

func (a *App) serve(s *http.Server, ln net.Listener) {
	if err := s.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		a.Log.Printf("server: %v", err)
	}
}

func (a *App) background(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	lastGame := input.Game{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		a.refreshBinds(false)
		g := a.Engine.Driver().Game()
		if g.Running != lastGame.Running || g.Focused != lastGame.Focused {
			a.broadcastOwners(map[string]any{"t": "game", "running": g.Running, "focused": g.Focused})
			a.Bus.Publish("game", g)
		}
		lastGame = g
		a.expireGrants()
	}
}

// Stop lets go of every key and closes the listeners.
func (a *App) Stop() {
	a.Engine.ReleaseAll(deck.ReasonReleaseAll)
	if a.tunnel != nil {
		a.tunnel.Stop()
	}
	if a.voice != nil {
		a.voice.stop()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, s := range []*http.Server{a.srv, a.guestSrv} {
		if s != nil {
			_ = s.Shutdown(ctx)
		}
	}
}

// ApplyConfig pushes changed settings to the running parts.
func (a *App) ApplyConfig(old, cfg config.Config) {
	a.Engine.SetSettings(engineSettings(cfg))
	a.Engine.Driver().SetGames(cfg.GameProcesses)
	if strings.Join(old.Actionmaps, "|") != strings.Join(cfg.Actionmaps, "|") || old.FollowGameBinds != cfg.FollowGameBinds ||
		strings.Join(old.GameDirs, "|") != strings.Join(cfg.GameDirs, "|") {
		a.refreshBinds(true)
	}
	if old.Layout != cfg.Layout || old.PhoneLayout != cfg.PhoneLayout {
		a.broadcastOwners(map[string]any{"t": "reload"})
	}
	if old.Voice != cfg.Voice || old.VoiceLanguage != cfg.VoiceLanguage || old.ShortEmotes != cfg.ShortEmotes ||
		old.VoiceEngine != cfg.VoiceEngine || old.ModelsDir != cfg.ModelsDir || old.SttModel != cfg.SttModel ||
		old.MicDevice != cfg.MicDevice || old.SpeakerDevice != cfg.SpeakerDevice || old.VoiceThreads != cfg.VoiceThreads ||
		(old.Replies == "off") != (cfg.Replies == "off") {
		a.voice.restart()
	} else if old.PttKey != cfg.PttKey || old.PttMode != cfg.PttMode || old.PttSound != cfg.PttSound {
		a.voice.sendPTT()
	}
	if old.TtsVoice != cfg.TtsVoice {
		if n := a.voice.Native(); n != nil && n.sp != nil {
			_ = n.sp.SetVoice(cfg.TtsVoice)
		}
	}
	if old.Tunnel != cfg.Tunnel {
		if cfg.Tunnel == "" {
			a.tunnel.Stop()
		} else {
			go a.tunnel.Start()
		}
	}
	if old.Discovery != cfg.Discovery || old.Name != cfg.Name {
		a.finder.announce()
	}
}

// Afk reports whether the owner said they're away, which opens AFK-only share links.
func (a *App) Afk() bool { return a.afk.Load() }

// SetAfk sets away mode and tells everyone who cares.
func (a *App) SetAfk(on bool) {
	if a.afk.Swap(on) == on {
		return
	}
	a.Log.Printf("Away mode %s.", map[bool]string{true: "on: AFK share links work now", false: "off"}[on])
	a.broadcast(map[string]any{"t": "afk", "on": on, "name": a.Store.Config().Name})
	a.Bus.Publish("afk", map[string]any{"on": on})
}
