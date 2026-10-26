package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xattribution/gorelaxor/conformer"

	"github.com/xattribution/tablet-deck-spooned/internal/audio"
	"github.com/xattribution/tablet-deck-spooned/internal/deck"
	"github.com/xattribution/tablet-deck-spooned/internal/layout"
	"github.com/xattribution/tablet-deck-spooned/internal/speech"
)

// nativeVoice is the gorelaxor voice: the microphone is always open (or opens for
// push-to-talk), MarbleNet cuts it into utterances, Parakeet or Whisper writes them
// down, and "Computer, <command>" presses a button. Pocket TTS answers.
type nativeVoice struct {
	a      *App
	models speech.Models
	rec    speech.Recognizer
	vad    *conformer.VAD
	sp     *speech.Speaker
	lis    *speech.Listener
	mic    audio.Recorder

	mu       sync.Mutex
	out      audio.Player
	phrases  map[string]string // phrase -> button id
	list     []string
	armed    time.Time // "Computer" alone: the next utterance within a few seconds is a command
	owner    *client
	stopPTT  chan struct{}
	lastText string
}

// modelDirs are where model files are looked for: the setting, then a "models"
// folder in the settings folder and next to Tablet Deck.
func (a *App) modelDirs() []string {
	dirs := []string{a.Store.Config().ModelsDir, filepath.Join(a.Store.Dir, "models")}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "models"))
	}
	return dirs
}

// FindSpeechModels lists the speech models Tablet Deck can see.
func (a *App) FindSpeechModels() speech.Models { return speech.FindModels(a.modelDirs()...) }

func voiceThreads(n int) int {
	if n > 0 {
		return n
	}
	return max(1, min(4, runtime.NumCPU()/4))
}

func startNative(a *App, l *layout.Layout) (*nativeVoice, error) {
	cfg := a.Store.Config()
	models := a.FindSpeechModels()
	stt := models.PickSTT(cfg.SttModel)
	if stt == "" {
		return nil, fmt.Errorf("no speech-to-text model found (looked in %s)", strings.Join(nonEmpty(a.modelDirs()), ", "))
	}
	threads := voiceThreads(cfg.VoiceThreads)
	n := &nativeVoice{a: a, models: models}
	t0 := time.Now()
	rec, err := speech.LoadRecognizer(stt, threads)
	if err != nil {
		return nil, err
	}
	n.rec = rec
	if models.VAD != "" {
		if n.vad, err = conformer.LoadVAD(models.VAD); err != nil {
			a.Log.Printf("Voice: MarbleNet didn't load (%v); using a loudness gate.", err)
		}
	}
	if models.TTS != "" && cfg.Replies != "off" || models.TTS != "" && cfg.ReadRequests {
		if n.sp, err = speech.NewSpeaker(models.TTS, models.Voices, threads); err != nil {
			a.Log.Printf("Voice: Pocket TTS didn't load (%v); no spoken replies.", err)
			n.sp = nil
		} else {
			_ = n.sp.SetVoice(cfg.TtsVoice)
			n.sp.SetOutput(n.play, n.waitPlayed, func(talking bool) {
				if n.lis != nil {
					if talking {
						n.lis.Mute(true)
					} else {
						time.AfterFunc(250*time.Millisecond, func() { n.lis.Mute(false) })
					}
				}
			})
		}
	}
	n.setPhrases(l, cfg.ShortEmotes)
	n.lis = speech.NewListener(n.vad, threads, n.utterance)
	if n.mic, err = audio.Record(cfg.MicDevice, speech.SampleRate, 50, n.feed); err != nil {
		if err != audio.ErrUnsupported {
			n.rec.Close()
			return nil, fmt.Errorf("the microphone didn't open: %w", err)
		}
		a.Log.Printf("Voice: no microphone on this system; commands only come from tests.")
		n.mic = nil
	}
	n.stopPTT = make(chan struct{})
	go n.pollPTTKey()
	a.Log.Printf("Voice (gorelaxor): %s, %s, TTS %s, loaded in %v. Say %q.", rec.Name(),
		map[bool]string{true: "MarbleNet", false: "loudness gate"}[n.vad != nil],
		map[bool]string{true: cfg.TtsVoice, false: "off"}[n.sp != nil], time.Since(t0).Round(time.Millisecond), cfg.WakeWord)
	return n, nil
}

func nonEmpty(list []string) []string {
	var out []string
	for _, s := range list {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (n *nativeVoice) setPhrases(l *layout.Layout, short bool) {
	phrases, _ := l.VoicePhrases(short)
	list := make([]string, 0, len(phrases))
	for p := range phrases {
		list = append(list, p)
	}
	sort.Strings(list)
	n.mu.Lock()
	n.phrases, n.list = phrases, list
	n.mu.Unlock()
	// Whisper reads a prompt of words to expect; button names help it spell them.
	hint := "Computer. " + strings.Join(list[:min(len(list), 60)], ", ")
	n.rec.SetHints(hint)
}

func (n *nativeVoice) feed(pcm []float32) {
	cfg := n.a.Store.Config()
	n.mu.Lock()
	ptt := n.owner != nil
	n.mu.Unlock()
	if !cfg.AlwaysListen && !ptt && !pttKeyDown(cfg.PttKey) {
		n.lis.Feed(make([]float32, len(pcm))) // keep time moving, hear nothing
		return
	}
	n.lis.Feed(pcm)
}

func (n *nativeVoice) close() {
	if n.stopPTT != nil {
		close(n.stopPTT)
	}
	if n.mic != nil {
		_ = n.mic.Close()
	}
	n.mu.Lock()
	if n.out != nil {
		_ = n.out.Close()
		n.out = nil
	}
	n.mu.Unlock()
	n.rec.Close()
}

// ---------- speaking ----------

func (n *nativeVoice) play(pcm []float32) error {
	n.mu.Lock()
	if n.out == nil {
		p, err := audio.NewPlayer(n.a.Store.Config().SpeakerDevice, n.sp.SampleRate())
		if err != nil {
			n.mu.Unlock()
			return err
		}
		n.out = p
	}
	out := n.out
	n.mu.Unlock()
	return out.Write(pcm)
}

func (n *nativeVoice) waitPlayed() {
	n.mu.Lock()
	out := n.out
	n.mu.Unlock()
	if out != nil {
		out.Wait()
	}
}

// say speaks a reply when replies are on (full replies also say what went wrong).
func (n *nativeVoice) say(text string, full bool) {
	cfg := n.a.Store.Config()
	if n.sp == nil || cfg.Replies == "off" || (full && cfg.Replies != "full") {
		return
	}
	n.sp.Say(text)
}

// announce speaks regardless of the replies setting (crew requests).
func (n *nativeVoice) announce(text string) {
	if n.sp != nil {
		n.sp.Say(text)
	}
}

// ---------- push-to-talk ----------

// listen is the deck's hold-to-talk: everything said while it's held is a command.
func (n *nativeVoice) listen(c *client, on bool) {
	n.mu.Lock()
	if on {
		n.owner = c
	} else if n.owner != c {
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()
	n.lis.SetPTT(on)
	if !on {
		// The owner stays set until the utterance is handled, so results go to that deck.
		time.AfterFunc(5*time.Second, func() {
			n.mu.Lock()
			if n.owner == c {
				n.owner = nil
			}
			n.mu.Unlock()
		})
	}
}

func (n *nativeVoice) pollPTTKey() {
	t := time.NewTicker(30 * time.Millisecond)
	defer t.Stop()
	was := false
	for {
		select {
		case <-n.stopPTT:
			return
		case <-t.C:
		}
		cfg := n.a.Store.Config()
		down := pttKeyDown(cfg.PttKey)
		if cfg.PttMode == "toggle" {
			if down && !was {
				n.togglePTT()
			}
		} else if down != was {
			n.lis.SetPTT(down)
			n.a.broadcastOwners(map[string]any{"t": "listening", "on": down})
		}
		was = down
	}
}

var pttToggled bool

func (n *nativeVoice) togglePTT() {
	pttToggled = !pttToggled
	n.lis.SetPTT(pttToggled)
	n.a.broadcastOwners(map[string]any{"t": "listening", "on": pttToggled})
}

// ---------- understanding ----------

func (n *nativeVoice) reply(m map[string]any) {
	n.mu.Lock()
	owner := n.owner
	n.mu.Unlock()
	if owner != nil {
		owner.send(m)
	} else {
		n.a.broadcastOwners(m)
	}
}

// utterance runs on the listener's goroutine for each thing said.
func (n *nativeVoice) utterance(pcm []float32, ptt bool) {
	a := n.a
	cfg := a.Store.Config()
	t0 := time.Now()
	text, err := n.rec.Transcribe(pcm)
	if err != nil {
		a.Log.Printf("Voice: %v", err)
		return
	}
	took := time.Since(t0)
	if strings.TrimSpace(text) == "" {
		return
	}
	cmd, woke := speech.SplitWake(text, cfg.WakeWord)
	n.mu.Lock()
	armed := time.Now().Before(n.armed)
	n.armed = time.Time{}
	n.mu.Unlock()
	if !ptt && !woke && !armed {
		return // talk that wasn't for us
	}
	a.Bus.Publish("heard", map[string]any{"text": text, "ms": took.Milliseconds(), "seconds": float64(len(pcm)) / speech.SampleRate})
	if cmd == "" {
		if woke { // "Computer." ... "landing gear"
			n.mu.Lock()
			n.armed = time.Now().Add(6 * time.Second)
			n.mu.Unlock()
			n.say("Yes?", false)
		}
		return
	}
	a.Log.Printf("Voice: %q (%d ms)", text, took.Milliseconds())
	if n.intent(cmd) {
		return
	}
	n.mu.Lock()
	list, phrases := n.list, n.phrases
	n.mu.Unlock()
	best, score := speech.Match(cmd, list)
	need := 0.55 + 0.35*cfg.VoiceConfidence // strictness 0.6 asks for 0.76
	if best == "" || score < need {
		a.Bus.Publish("heard", map[string]any{"text": text, "accepted": false, "best": best, "score": score})
		n.reply(map[string]any{"t": "unheard"})
		n.say("Say again?", false)
		return
	}
	l, err := a.Layout("tablet", "")
	if err != nil {
		return
	}
	b := l.Buttons[phrases[best]]
	if b == nil {
		return
	}
	a.Bus.Publish("heard", map[string]any{"text": text, "label": b.Label, "confidence": score, "accepted": true})
	n.reply(map[string]any{"t": "heard", "id": b.ID, "label": b.Label, "text": text})
	ms := b.VoiceMs
	if ms == 0 {
		ms = b.Ms
	}
	if r := a.Engine.Tap(b, sourceVoice(), ms, "voice"); !r.OK {
		n.reply(map[string]any{"t": "blocked", "id": b.ID, "reason": r.Reason})
		n.say(spokenReason(r.Reason), false)
		return
	}
	n.say(b.Label, false)
}

func spokenReason(r string) string {
	switch r {
	case deck.ReasonNotRunning:
		return "Star Citizen isn't running."
	case deck.ReasonNotFocused, deck.ReasonFocusFailed:
		return "Star Citizen isn't in front."
	}
	return "That didn't go through."
}

var (
	reRelease = regexp.MustCompile(`^(release|let go of|drop)( all)?( the)?( keys| buttons)?$|^let go$`)
	reAfkOn   = regexp.MustCompile(`^(i'?m |i am |going |go )?(afk|away|stepping away|be right back|brb)$|^away mode on$`)
	reAfkOff  = regexp.MustCompile(`^(i'?m |i am )?back$|^away mode off$|^i'?m here$`)
	reTell    = regexp.MustCompile(`^(tell|ask|message|send) (\S+)( that)? (.+)$`)
	reStatus  = regexp.MustCompile(`^(status|report|sitrep|status report|what'?s (my|the) status)$`)
	reCancel  = regexp.MustCompile(`^(cancel|never mind|nevermind|nothing|stop|quiet|shut up)$`)
	reLights  = regexp.MustCompile(`^reset( the)?( button)? lights$`)
)

// intent handles the built-in commands; it reports whether it did.
func (n *nativeVoice) intent(cmd string) bool {
	a := n.a
	cmd = strings.TrimSpace(cmd)
	switch {
	case reCancel.MatchString(cmd):
		return true
	case reRelease.MatchString(cmd):
		a.Engine.ReleaseAll(deck.ReasonReleaseAll)
		n.say("Keys released.", false)
		return true
	case reAfkOn.MatchString(cmd):
		a.SetAfk(true)
		n.say("Away mode on. Your crew has the controls.", false)
		return true
	case reAfkOff.MatchString(cmd):
		a.SetAfk(false)
		n.say("Welcome back.", false)
		return true
	case reLights.MatchString(cmd):
		a.Engine.ResetStates()
		n.say("Lights reset.", false)
		return true
	case reStatus.MatchString(cmd):
		n.say(a.spokenStatus(), false)
		return true
	}
	if m := reTell.FindStringSubmatch(cmd); m != nil {
		name, text := m[2], m[4]
		best, bestScore := "", 0.0
		for _, c := range a.crew.summary() {
			who, _ := c["name"].(string)
			first := strings.Fields(strings.ToLower(who))
			if len(first) == 0 {
				continue
			}
			if _, sc := speech.Match(name, []string{first[0]}); sc > bestScore {
				best, bestScore = c["id"].(string), sc
			}
		}
		if best == "" || bestScore < 0.7 {
			n.say("I don't see "+name+" in your crew.", false)
			return true
		}
		a.crew.request(best, text, "", &client{a: a, out: make(chan []byte, 4)})
		n.say("Sent.", false)
		return true
	}
	return false
}

// spokenStatus is a short status report for "Computer, status".
func (a *App) spokenStatus() string {
	g := a.Engine.Driver().Game()
	var parts []string
	switch {
	case !g.Running:
		parts = append(parts, "Star Citizen isn't running.")
	case g.Focused:
		parts = append(parts, "Star Citizen is in front.")
	default:
		parts = append(parts, "Star Citizen is running in the background.")
	}
	decks, crew := 0, 0
	a.each(func(c *client) {
		if c.p.grant == nil {
			decks++
		} else {
			crew++
		}
	})
	parts = append(parts, fmt.Sprintf("%s connected.", count(decks, "deck")))
	if crew > 0 {
		parts = append(parts, fmt.Sprintf("%s linked.", count(crew, "crewmate")))
	}
	if a.Afk() {
		parts = append(parts, "Away mode is on.")
	}
	if q := len(a.reqs.open()); q > 0 {
		parts = append(parts, count(q, "request")+" waiting.")
	}
	return strings.Join(parts, " ")
}

func count(n int, what string) string {
	words := []string{"No", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten"}
	w := fmt.Sprint(n)
	if n < len(words) {
		w = words[n]
	}
	if n != 1 {
		what += "s"
	}
	return w + " " + what
}
