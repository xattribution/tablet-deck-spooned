// Command tabletdeck is Tablet Deck: a touch control deck for Star Citizen on any
// tablet or phone, with crew sharing and Stream Deck support.
//
//	tabletdeck                    start (tray icon on Windows) and open the control panel
//	tabletdeck --tray             start in the tray only (Start with Windows)
//	tabletdeck serve [--dry-run]  run the server in a console, no tray
//	tabletdeck press <button>     press a button on the running deck (Stream Deck, scripts)
//	tabletdeck url tabletdeck://press/<button>
//	tabletdeck release-all        let go of every held key
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tabletdeck "github.com/xattribution/tablet-deck-spooned"
	"github.com/xattribution/tablet-deck-spooned/internal/app"
	"github.com/xattribution/tablet-deck-spooned/internal/config"
	"github.com/xattribution/tablet-deck-spooned/internal/deck"
	"github.com/xattribution/tablet-deck-spooned/internal/sc"
	"github.com/xattribution/tablet-deck-spooned/internal/winapp"
)

func main() {
	winapp.AttachConsole()
	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "", "run":
		run(args, true)
	case "serve":
		run(args, false)
	case "press", "down", "up":
		os.Exit(press(cmd, args))
	case "url":
		os.Exit(handleURL(args))
	case "release-all":
		os.Exit(call("POST", "/api/v1/release-all", nil, nil))
	case "mcp":
		os.Exit(mcpStdio())
	case "game-actions":
		os.Exit(gameActions(args))
	case "version":
		fmt.Println(tabletdeck.Version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
}

const usage = `Tablet Deck: a touch control deck for Star Citizen.

  tabletdeck                       start, with the tray icon, and open the control panel
  tabletdeck --tray                start in the tray only
  tabletdeck serve [--dry-run]     run in this console without the tray
         [--port N] [--data DIR]   (--dry-run presses no keys)
  tabletdeck press <button>        press a button on the running deck, by id or label
         [--action NAME]           ... or by Star Citizen action (v_toggle_landing_system)
         [--keys COMBO]            ... or a raw key combo (lalt+n)
  tabletdeck down|up <button>      hold or let go of a button (Stream Deck hold keys)
  tabletdeck url tabletdeck://press/<button>
  tabletdeck release-all           let go of every key Tablet Deck holds
  tabletdeck mcp                   MCP server on stdio, for AI agents (bridges to the running deck)
`

// run starts the deck. With tray (Windows) it lives in the notification area.
func run(args []string, tray bool) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	dry := fs.Bool("dry-run", false, "press no keys, only log them")
	port := fs.Int("port", 0, "port for tablets (default from config.json)")
	data := fs.String("data", "", "settings folder")
	trayOnly := fs.Bool("tray", false, "start in the tray without opening the control panel")
	_ = fs.Parse(args)

	// One tray app per settings folder. Starting it again opens the running one's
	// control panel. A console run ("serve") is for testing and doesn't take part.
	if tray && !winapp.SingleInstance(*data) {
		if !*trayOnly {
			openPanel(loadCfg(*data), "")
		}
		return
	}
	dir := *data
	if dir == "" {
		dir = config.DataDir()
	}
	_ = os.MkdirAll(dir, 0o755)
	logger := newLogger(dir, !tray)
	a, err := app.New(app.Options{DryRun: *dry, DataDir: dir, Port: *port, Log: logger})
	if err != nil {
		fail(dir, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := a.Start(ctx); err != nil {
		fail(dir, err)
	}
	_ = winapp.RegisterLinks()
	cfg := a.Store.Config()

	stop := func() {
		a.Stop()
		cancel()
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	if !tray {
		printWelcome(a)
		<-sig
		stop()
		return
	}
	go func() { <-sig; stop(); os.Exit(0) }()
	if !*trayOnly {
		openPanel(cfg, "")
	}
	winapp.RunTray(winapp.Tray{
		Title:       "Tablet Deck",
		OpenPanel:   func() { openPanel(a.Store.Config(), "") },
		OpenPairing: func() { openPanel(a.Store.Config(), "#pair") },
		OpenTouch: func() {
			c := a.Store.Config()
			winapp.OpenWindow(fmt.Sprintf("http://localhost:%d/?t=%s&device=tablet&name=This%%20PC", c.Port, c.Token), "touch", 1024, 768)
		},
		ReleaseAll: func() { a.Engine.ReleaseAll(deck.ReasonReleaseAll) },
		ToggleAfk:  func() bool { a.SetAfk(!a.Afk()); return a.Afk() },
		Quit:       stop,
	})
}

func openPanel(c config.Config, hash string) {
	winapp.OpenWindow(fmt.Sprintf("http://localhost:%d/control?k=%s%s", c.Port, c.APIKey, hash), "panel", 1180, 820)
}

func printWelcome(a *app.App) {
	cfg := a.Store.Config()
	fmt.Printf("\nOpen this on your tablet or phone (same network):\n")
	for _, ad := range app.Addresses() {
		note := ""
		if ad.Virtual {
			note = "   (VPN or virtual adapter)"
		}
		fmt.Printf("   %-22s http://%s:%d/?t=%s%s\n", ad.Adapter+":", ad.IP, cfg.Port, cfg.Token, note)
	}
	fmt.Printf("\nPairing code: %s\nControl panel (this PC): http://localhost:%d/control?k=%s\n", cfg.Token, cfg.Port, cfg.APIKey)
	fmt.Printf("Settings: %s\n\n", a.Store.ConfigPath())
}

func newLogger(dir string, console bool) *log.Logger {
	path := filepath.Join(dir, "tabletdeck.log")
	if fi, err := os.Stat(path); err == nil && fi.Size() > 2<<20 {
		_ = os.Rename(path, path+".old")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	var w io.Writer = os.Stdout
	if err == nil {
		if console {
			w = io.MultiWriter(os.Stdout, f)
		} else {
			w = f
		}
	}
	return log.New(w, "", log.LstdFlags)
}

func fail(dir string, err error) {
	msg := err.Error()
	_ = os.WriteFile(filepath.Join(dir, "crash.log"), []byte(time.Now().Format(time.RFC3339)+" "+msg+"\n"), 0o644)
	fmt.Fprintln(os.Stderr, msg)
	winapp.Message("Tablet Deck", "Tablet Deck couldn't start:\n\n"+msg)
	os.Exit(1)
}

// ---------- talking to the running deck ----------

func loadCfg(dir string) config.Config {
	if dir == "" {
		dir = config.DataDir()
	}
	var c config.Config
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err == nil {
		_ = json.Unmarshal(b, &c)
	}
	if c.Port == 0 {
		c.Port = config.DefaultPort
	}
	return c
}

// call sends one request to the running deck's API with its API key.
func call(method, path string, body any, out any) int {
	c := loadCfg(os.Getenv("TABLETDECK_DATA"))
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, fmt.Sprintf("http://127.0.0.1:%d%s", c.Port, path), rd)
	req.Header.Set("X-Deck-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Tablet Deck isn't running:", err)
		return 1
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if out != nil {
		_ = json.Unmarshal(b, out)
	}
	fmt.Println(strings.TrimSpace(string(b)))
	if resp.StatusCode >= 300 {
		return 1
	}
	return 0
}

func press(mode string, args []string) int {
	fs := flag.NewFlagSet("press", flag.ExitOnError)
	action := fs.String("action", "", "Star Citizen action name")
	keys := fs.String("keys", "", "raw key combo")
	ms := fs.Int("ms", 0, "how long to hold, for a tap")
	layout := fs.String("layout", "", "layout file to find the button in")
	who := fs.String("who", "Command line", "shown in the deck's log")
	// Flags may come after the button name.
	var name string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args = args[0], args[1:]
	}
	_ = fs.Parse(args)
	if name == "" && fs.NArg() > 0 {
		name = fs.Arg(0)
	}
	if name == "" && *action == "" && *keys == "" {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	if mode == "press" {
		mode = "tap"
	}
	body := map[string]any{"id": name, "label": name, "action": *action, "keys": *keys, "mode": mode, "ms": *ms, "layout": *layout, "who": *who}
	var res struct {
		OK bool `json:"ok"`
	}
	if code := call("POST", "/api/v1/press", body, &res); code != 0 || !res.OK {
		return 1
	}
	return 0
}

// handleURL runs a tabletdeck:// link: press/<button>, down/<button>, up/<button>,
// action/<name>, release-all, open.
func handleURL(args []string) int {
	if len(args) == 0 {
		return 2
	}
	u, err := url.Parse(args[0])
	if err != nil || u.Scheme != "tabletdeck" {
		return 2
	}
	verb := u.Host
	arg := strings.Trim(u.Path, "/")
	arg, _ = url.PathUnescape(arg)
	switch verb {
	case "press", "down", "up":
		return press(verb, []string{arg})
	case "action":
		return press("press", []string{"--action", arg})
	case "release-all":
		return call("POST", "/api/v1/release-all", nil, nil)
	case "afk":
		return call("POST", "/api/v1/afk", map[string]bool{"on": arg != "off"}, nil)
	case "open":
		openPanel(loadCfg(""), "")
		return 0
	}
	return 2
}

// mcpStdio bridges MCP over stdio (one JSON-RPC message per line) to the running
// deck's /mcp endpoint, for agents that launch a command (Claude Desktop and others).
func mcpStdio() int {
	c := loadCfg(os.Getenv("TABLETDECK_DATA"))
	client := &http.Client{Timeout: 30 * time.Second}
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	for in.Scan() {
		line := bytes.TrimSpace(in.Bytes())
		if len(line) == 0 {
			continue
		}
		req, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/mcp", c.Port), bytes.NewReader(line))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Deck-Key", c.APIKey)
		resp, err := client.Do(req)
		if err != nil {
			var id json.RawMessage
			var probe struct {
				ID json.RawMessage `json:"id"`
			}
			if json.Unmarshal(line, &probe) == nil {
				id = probe.ID
			}
			if len(id) > 0 {
				b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32000, "message": "Tablet Deck isn't running: start it, then try again"}})
				out.Write(append(b, '\n'))
				out.Flush()
			}
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusAccepted || len(bytes.TrimSpace(body)) == 0 {
			continue
		}
		out.Write(append(bytes.TrimSpace(body), '\n'))
		out.Flush()
	}
	return 0
}

// gameActions prints every action of a Star Citizen install (a channel folder such as
// "C:\\Program Files\\Roberts Space Industries\\StarCitizen\\LIVE") as JSON.
func gameActions(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: tabletdeck game-actions <channel folder>")
		return 2
	}
	acts, version, err := sc.LoadActions(args[0], "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "%d actions, game %s\n", len(acts), version)
	_ = json.NewEncoder(os.Stdout).Encode(acts)
	return 0
}
