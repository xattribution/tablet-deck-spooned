package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xattribution/tablet-deck-spooned/internal/layout"
)

// voice runs voice.exe (native/voice.cs), which listens with Windows' offline speech
// recognition for the layout's phrases while a talk button or the push-to-talk key is
// held. It's a separate process because System.Speech is .NET only.
type voice struct {
	a *App

	mu       sync.Mutex
	proc     *exec.Cmd
	stdin    io.WriteCloser
	ready    bool
	state    map[string]any // what decks are told: {state: off|starting|ready|unavailable, reason}
	commands [][2]string    // [phrase, button label], for the deck's list
	owner    *client        // the deck holding the talk button gets the result
	fromKey  bool           // the last listening came from the PC's push-to-talk key
	gen      int            // bumps on restart, so an old process's exit is ignored
	nat      *nativeVoice   // the gorelaxor engine, when it's the one running
	engine   string         // native or windows, once started
}

func newVoice(a *App) *voice { return &voice{a: a, state: map[string]any{"state": "off"}} }

func (v *voice) status() map[string]any {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := map[string]any{}
	for k, x := range v.state {
		out[k] = x
	}
	return out
}

func (v *voice) setState(state, reason string) {
	v.mu.Lock()
	v.state = map[string]any{"state": state}
	if reason != "" {
		v.state["reason"] = reason
	}
	st := v.state
	v.mu.Unlock()
	v.a.broadcastOwners(map[string]any{"t": "voice-status", "state": st["state"], "reason": st["reason"]})
	v.a.Bus.Publish("voice", st)
}

// builtinCommands are what the gorelaxor engine understands besides the buttons.
var builtinCommands = [][2]string{
	{"release all keys", "Release all keys"}, {"i'm afk", "Away mode on"}, {"i'm back", "Away mode off"},
	{"tell <crewmate> <message>", "Send a crew request"}, {"status", "Status report"}, {"reset lights", "Reset button lights"},
}

// say speaks a line with the gorelaxor engine's voice, if it's running.
func (v *voice) say(text string) {
	v.mu.Lock()
	n := v.nat
	v.mu.Unlock()
	if n != nil && v.a.Store.Config().ReadRequests {
		n.announce(text)
	}
}

// Native is the gorelaxor engine, nil when it isn't running.
func (v *voice) Native() *nativeVoice {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.nat
}

// Commands lists what can be said.
func (v *voice) Commands() [][2]string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([][2]string(nil), v.commands...)
}

// voiceExe finds voice.exe next to Tablet Deck; running from source on Windows, it's
// compiled from native/voice.cs with the C# compiler that ships with Windows.
func voiceExe() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("voice commands need Windows")
	}
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	for _, p := range []string{filepath.Join(dir, "voice.exe"), filepath.Join(dir, "native", "voice.exe")} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	src := filepath.Join("native", "voice.cs")
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("voice.exe is missing; reinstall Tablet Deck")
	}
	out := filepath.Join("native", "voice.exe")
	fw := filepath.Join(os.Getenv("WINDIR"), "Microsoft.NET", "Framework64", "v4.0.30319")
	cmd := exec.Command(filepath.Join(fw, "csc.exe"), "/nologo", "/optimize", "/target:exe", "/platform:x64",
		"/lib:"+filepath.Join(fw, "WPF"), "/r:System.Speech.dll", "/out:"+out, src)
	hideWindow(cmd)
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("voice.cs didn't compile: %s", strings.TrimSpace(string(b)))
	}
	return out, nil
}

func (v *voice) start() {
	cfg := v.a.Store.Config()
	if !cfg.Voice {
		v.setState("off", "")
		return
	}
	l, err := v.a.Layout("tablet", "")
	if err != nil {
		v.setState("unavailable", err.Error())
		return
	}
	phrases, warnings := l.VoicePhrases(cfg.ShortEmotes)
	for _, w := range warnings {
		v.a.Log.Print(w)
	}
	cmds := make([][2]string, 0, len(phrases))
	for p, id := range phrases {
		cmds = append(cmds, [2]string{p, l.Buttons[id].Label})
	}
	sort.Slice(cmds, func(i, j int) bool {
		if cmds[i][1] != cmds[j][1] {
			return cmds[i][1] < cmds[j][1]
		}
		return cmds[i][0] < cmds[j][0]
	})

	// The gorelaxor engine when its models are there (or asked for), else Windows'.
	if cfg.VoiceEngine == "native" || (cfg.VoiceEngine != "windows" && len(v.a.FindSpeechModels().STT) > 0) {
		v.mu.Lock()
		v.gen++
		gen := v.gen
		v.commands = append(cmds, builtinCommands...)
		v.mu.Unlock()
		v.setState("starting", "")
		n, err := startNative(v.a, l)
		v.mu.Lock()
		stale := v.gen != gen
		if err == nil && !stale {
			v.nat, v.engine = n, "native"
		}
		v.mu.Unlock()
		if stale {
			if n != nil {
				n.close()
			}
			return
		}
		if err == nil {
			v.setState("ready", "")
			return
		}
		v.a.Log.Printf("Voice (gorelaxor) unavailable: %v", err)
		if cfg.VoiceEngine == "native" {
			v.setState("unavailable", err.Error())
			return
		}
	}
	exe, err := voiceExe()
	if err != nil {
		v.setState("unavailable", err.Error())
		return
	}
	cmd := exec.Command(exe, cfg.VoiceLanguage)
	hideWindow(cmd)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		v.setState("unavailable", err.Error())
		return
	}
	v.mu.Lock()
	v.gen++
	gen := v.gen
	v.proc, v.stdin, v.ready, v.commands, v.engine = cmd, stdin, false, cmds, "windows"
	v.mu.Unlock()
	v.setState("starting", "")
	for p, id := range phrases {
		fmt.Fprintf(stdin, "phrase %s %s\n", id, p)
	}
	fmt.Fprintln(stdin, "ready")

	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			v.line(sc.Text(), l)
		}
		err := cmd.Wait()
		v.mu.Lock()
		current := v.gen == gen
		wasReady := v.ready
		if current {
			v.proc, v.stdin, v.ready = nil, nil, false
		}
		v.mu.Unlock()
		if !current {
			return
		}
		if wasReady {
			v.setState("starting", "")
			time.Sleep(2 * time.Second)
			v.start()
		} else if st := v.status(); st["state"] != "unavailable" {
			v.setState("unavailable", fmt.Sprintf("the voice listener exited (%v)", err))
		}
	}()
}

func (v *voice) line(line string, l *layout.Layout) {
	a := v.a
	switch {
	case strings.HasPrefix(line, "ready"):
		v.mu.Lock()
		v.ready = true
		n := len(v.commands)
		v.mu.Unlock()
		v.sendPTT()
		a.Log.Printf("Voice commands ready (%d phrases, %s).", n, strings.TrimSpace(strings.TrimPrefix(line, "ready")))
		v.setState("ready", "")
	case line == "ptt 1" || line == "ptt 0":
		v.mu.Lock()
		v.fromKey, v.owner = true, nil
		v.mu.Unlock()
		a.broadcastOwners(map[string]any{"t": "listening", "on": line == "ptt 1"})
	case strings.HasPrefix(line, "err"):
		v.setState("unavailable", strings.TrimSpace(strings.TrimPrefix(line, "err")))
	case strings.HasPrefix(line, "heard "):
		parts := strings.SplitN(line, " ", 4)
		if len(parts) < 4 {
			return
		}
		conf, _ := strconv.ParseFloat(parts[2], 64)
		v.heard(l, parts[1], conf, parts[3])
	case line == "none":
		v.heard(l, "", 0, "")
	}
}

func (v *voice) heard(l *layout.Layout, id string, conf float64, text string) {
	a := v.a
	v.mu.Lock()
	owner, fromKey := v.owner, v.fromKey
	v.mu.Unlock()
	reply := func(m map[string]any) {
		if fromKey || owner == nil {
			a.broadcastOwners(m)
		} else {
			owner.send(m)
		}
	}
	// The layout may have changed since the listener started; look the button up again.
	if cur, err := a.Layout("tablet", ""); err == nil {
		l = cur
	}
	b := l.Buttons[id]
	if b == nil || conf < a.Store.Config().VoiceConfidence {
		a.Bus.Publish("heard", map[string]any{"text": text, "confidence": conf, "accepted": false})
		reply(map[string]any{"t": "unheard"})
		return
	}
	a.Bus.Publish("heard", map[string]any{"text": text, "label": b.Label, "confidence": conf, "accepted": true})
	reply(map[string]any{"t": "heard", "id": b.ID, "label": b.Label, "text": text})
	ms := b.VoiceMs
	if ms == 0 {
		ms = b.Ms
	}
	if r := a.Engine.Tap(b, sourceVoice(), ms, "voice"); !r.OK {
		reply(map[string]any{"t": "blocked", "id": b.ID, "reason": r.Reason})
	}
}

// listen starts or stops listening for a deck holding its talk button.
func (v *voice) listen(c *client, on bool) {
	v.mu.Lock()
	if n := v.nat; n != nil {
		v.mu.Unlock()
		n.listen(c, on)
		return
	}
	ready, stdin := v.ready, v.stdin
	if on {
		v.owner, v.fromKey = c, false
	}
	isOwner := v.owner == c
	v.mu.Unlock()
	if !ready || stdin == nil {
		reason := "Voice commands are turned off"
		if v.a.Store.Config().Voice {
			reason = "Voice commands aren't running on the PC"
		}
		c.send(map[string]any{"t": "voice-off", "reason": reason})
		return
	}
	if on {
		fmt.Fprintln(stdin, "listen")
	} else if isOwner {
		fmt.Fprintln(stdin, "stop")
	}
}

// tap starts tap to talk (the native voice; the Windows voice treats it as a short hold).
func (v *voice) tap(c *client) {
	v.mu.Lock()
	n := v.nat
	v.mu.Unlock()
	if n != nil {
		n.tap(c)
		c.send(map[string]any{"t": "listening", "on": true})
		return
	}
	v.listen(c, true)
	time.AfterFunc(5*time.Second, func() { v.listen(c, false); c.send(map[string]any{"t": "listening", "on": false}) })
}

// stopFor stops listening when the deck holding the talk button goes away.
func (v *voice) stopFor(c *client) {
	if v == nil {
		return
	}
	v.mu.Lock()
	if n := v.nat; n != nil {
		v.mu.Unlock()
		n.listen(c, false)
		return
	}
	mine := v.owner == c
	if mine {
		v.owner = nil
	}
	stdin := v.stdin
	v.mu.Unlock()
	if mine && stdin != nil {
		fmt.Fprintln(stdin, "stop")
	}
}

func (v *voice) sendPTT() {
	v.mu.Lock()
	stdin := v.stdin
	v.mu.Unlock()
	if stdin == nil {
		return
	}
	cfg := v.a.Store.Config()
	mode := "hold"
	if cfg.PttMode == "toggle" {
		mode = "toggle"
	}
	fmt.Fprintf(stdin, "ptt %d %s\n", cfg.PttKey, mode)
	chime := "on"
	if cfg.PttSound != nil && !*cfg.PttSound {
		chime = "off"
	}
	fmt.Fprintf(stdin, "chime %s\n", chime)
}

func (v *voice) stop() {
	v.mu.Lock()
	v.gen++
	if v.nat != nil {
		v.nat.close()
		v.nat = nil
	}
	p := v.proc
	v.proc, v.stdin, v.ready = nil, nil, false
	v.mu.Unlock()
	if p != nil && p.Process != nil {
		_ = p.Process.Kill()
	}
}

func (v *voice) restart() {
	if v == nil {
		return
	}
	v.stop()
	go v.start()
}
