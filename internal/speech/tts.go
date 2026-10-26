package speech

import (
	"fmt"
	"strings"
	"sync"

	"github.com/xattribution/gorelaxor/pockettts"
)

// Speaker reads text aloud with Pocket TTS, one line after another.
type Speaker struct {
	m       *pockettts.Model
	files   map[string]string
	threads int

	mu     sync.Mutex
	voices map[string]*pockettts.Voice
	voice  string
	queue  chan string
	play   func(pcm []float32) error // plays samples at SampleRate()
	wait   func()
	onTalk func(talking bool)
}

// NewSpeaker loads the TTS model; voices load when first used.
func NewSpeaker(model string, voices map[string]string, threads int) (*Speaker, error) {
	m, err := pockettts.Load(model)
	if err != nil {
		return nil, err
	}
	s := &Speaker{m: m, files: voices, threads: threads, voices: map[string]*pockettts.Voice{}, queue: make(chan string, 8)}
	go s.loop()
	return s, nil
}

// SampleRate of the speech (24 kHz).
func (s *Speaker) SampleRate() int { return s.m.SampleRate() }

// SetOutput sets where audio goes: play gets each piece, wait blocks until it has
// all played. onTalk is told when speaking starts and stops (to mute the mic).
func (s *Speaker) SetOutput(play func([]float32) error, wait func(), onTalk func(bool)) {
	s.mu.Lock()
	s.play, s.wait, s.onTalk = play, wait, onTalk
	s.mu.Unlock()
}

// SetVoice picks a voice by name ("alba", "george").
func (s *Speaker) SetVoice(name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if _, ok := s.files[name]; !ok {
		return fmt.Errorf("no voice %q", name)
	}
	s.mu.Lock()
	s.voice = name
	s.mu.Unlock()
	return nil
}

func (s *Speaker) loadVoice(name string) (*pockettts.Voice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v := s.voices[name]; v != nil {
		return v, nil
	}
	f, ok := s.files[name]
	if !ok {
		for n, p := range s.files { // any voice, if the chosen one is gone
			name, f, ok = n, p, true
			break
		}
	}
	if !ok {
		return nil, fmt.Errorf("no voice files found")
	}
	v, err := s.m.LoadVoice(f)
	if err != nil {
		return nil, err
	}
	s.voices[name] = v
	return v, nil
}

// Say queues a line. Lines that pile up (more than a few) are dropped: a late
// answer is worse than none.
func (s *Speaker) Say(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	select {
	case s.queue <- text:
	default:
	}
}

// Synthesize returns a line's audio, for previews and tests.
func (s *Speaker) Synthesize(text, voice string) ([]float32, error) {
	v, err := s.loadVoice(strings.ToLower(voice))
	if err != nil {
		return nil, err
	}
	return s.m.Synthesize(ForSpeech(text), v, pockettts.Options{Threads: s.threads})
}

func (s *Speaker) loop() {
	for text := range s.queue {
		s.mu.Lock()
		play, wait, onTalk, name := s.play, s.wait, s.onTalk, s.voice
		s.mu.Unlock()
		if play == nil {
			continue
		}
		v, err := s.loadVoice(name)
		if err != nil {
			continue
		}
		if onTalk != nil {
			onTalk(true)
		}
		_ = s.m.Speak(ForSpeech(text), v, pockettts.Options{Threads: s.threads}, play)
		if wait != nil {
			wait()
		}
		if onTalk != nil {
			onTalk(false)
		}
	}
}

var spokenDigits = []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}

// ForSpeech rewrites text for the ear: Pocket TTS reads digits one by one, so
// small numbers are spelled out, and symbols become words.
func ForSpeech(s string) string {
	r := strings.NewReplacer("+", " up", "&", " and ", "%", " percent", "/", " ", "_", " ")
	s = r.Replace(s)
	var b strings.Builder
	for _, w := range strings.Fields(s) {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		if len(w) == 1 && w[0] >= '0' && w[0] <= '9' {
			b.WriteString(spokenDigits[w[0]-'0'])
			continue
		}
		b.WriteString(w)
	}
	return b.String()
}

// Resample converts mono audio between sample rates with linear interpolation,
// which is plenty for speech going into a recognizer.
func Resample(pcm []float32, from, to int) []float32 {
	if from == to || len(pcm) == 0 {
		return pcm
	}
	n := int(int64(len(pcm)) * int64(to) / int64(from))
	out := make([]float32, n)
	step := float64(from) / float64(to)
	for i := range out {
		x := float64(i) * step
		j := int(x)
		f := float32(x - float64(j))
		if j+1 < len(pcm) {
			out[i] = pcm[j]*(1-f) + pcm[j+1]*f
		} else {
			out[i] = pcm[len(pcm)-1]
		}
	}
	return out
}
