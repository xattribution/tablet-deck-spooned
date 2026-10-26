package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"
)

// tunnel puts the guest port on the internet with a Cloudflare quick tunnel
// (cloudflared, no account), so a share link works for friends anywhere. Only the
// guest port goes through it: share links, never the pairing code, the editor or
// the API.
type tunnel struct {
	a      *App
	mu     sync.Mutex
	url    string
	state  string // off, starting, up, error
	err    string
	cancel context.CancelFunc
}

func newTunnel(a *App) *tunnel { return &tunnel{a: a, state: "off"} }

var reTryCF = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

// cloudflaredPath finds cloudflared: next to Tablet Deck, in the data folder, or on PATH.
func (t *tunnel) cloudflaredPath() string {
	name := "cloudflared"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	dirs = append(dirs, t.a.Store.Dir)
	for _, d := range dirs {
		p := filepath.Join(d, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// Status reports the tunnel's state and its address while up.
func (t *tunnel) Status() map[string]any {
	t.mu.Lock()
	defer t.mu.Unlock()
	return map[string]any{"state": t.state, "url": t.url, "error": t.err, "installed": t.cloudflaredPath() != ""}
}

// URL is the tunnel's address, "" while it's down.
func (t *tunnel) URL() string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state != "up" {
		return ""
	}
	return t.url
}

func (t *tunnel) set(state, url, err string) {
	t.mu.Lock()
	t.state, t.url, t.err = state, url, err
	t.mu.Unlock()
	t.a.Bus.Publish("tunnel", t.Status())
}

// Start runs cloudflared and keeps it running until Stop, restarting it if it exits.
func (t *tunnel) Start() {
	t.mu.Lock()
	if t.cancel != nil {
		t.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	t.mu.Unlock()
	for ctx.Err() == nil {
		bin := t.cloudflaredPath()
		if bin == "" {
			t.set("error", "", "cloudflared isn't installed. Get it from github.com/cloudflare/cloudflared/releases (cloudflared-windows-amd64.exe), rename it cloudflared.exe and put it in "+t.a.Store.Dir)
			select {
			case <-ctx.Done():
			case <-time.After(30 * time.Second):
			}
			continue
		}
		t.set("starting", "", "")
		err := t.run(ctx, bin)
		if ctx.Err() != nil {
			break
		}
		msg := "the tunnel stopped"
		if err != nil {
			msg = err.Error()
		}
		t.set("error", "", msg+"; retrying")
		select {
		case <-ctx.Done():
		case <-time.After(10 * time.Second):
		}
	}
	t.set("off", "", "")
}

func (t *tunnel) run(ctx context.Context, bin string) error {
	port := t.a.Store.Config().GuestPort
	cmd := exec.CommandContext(ctx, bin, "tunnel", "--no-autoupdate", "--url", fmt.Sprintf("http://127.0.0.1:%d", port))
	hideWindow(cmd)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	cmd.Stdout = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(stderr)
	for sc.Scan() {
		if u := reTryCF.FindString(sc.Text()); u != "" && t.URL() == "" {
			t.set("up", u, "")
			t.a.Log.Printf("Share links now work from anywhere: %s", u)
			t.a.broadcastOwners(map[string]any{"t": "tunnel", "url": u})
		}
	}
	return cmd.Wait()
}

// Stop ends the tunnel.
func (t *tunnel) Stop() {
	t.mu.Lock()
	c := t.cancel
	t.cancel = nil
	t.mu.Unlock()
	if c != nil {
		c()
	}
}
