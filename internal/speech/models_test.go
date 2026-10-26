package speech

import (
	"os"
	"testing"
	"time"

	"github.com/xattribution/gorelaxor/conformer"
)

// TestRoundTrip speaks commands with Pocket TTS, runs them through the listener and
// a recognizer, and checks the right button comes out. It needs real model files:
// TD_MODELS=<folder with the .gguf/.bin files> go test ./internal/speech -run RoundTrip
func TestRoundTrip(t *testing.T) {
	dir := os.Getenv("TD_MODELS")
	if dir == "" {
		t.Skip("TD_MODELS not set")
	}
	m := FindModels(dir)
	t.Logf("found: stt=%v vad=%s tts=%s voices=%d", m.STT, m.VAD, m.TTS, len(m.Voices))
	if len(m.STT) == 0 || m.TTS == "" || m.VAD == "" {
		t.Skip("models missing")
	}
	sp, err := NewSpeaker(m.TTS, m.Voices, 4)
	if err != nil {
		t.Fatal(err)
	}
	vad, err := conformer.LoadVAD(m.VAD)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	rec, err := LoadRecognizer(m.PickSTT(os.Getenv("TD_STT")), 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("loaded %s in %v", rec.Name(), time.Since(t0).Round(time.Millisecond))
	phrases := []string{"landing gear", "deploy landing gear", "shields up", "open star map", "engage quantum", "flight ready", "request landing", "emote wave"}
	rec.SetHints("Computer. Landing gear, shields, quantum, star map, flight ready.")

	for _, c := range []struct{ line, want string }{
		{"Computer, deploy the landing gear.", "deploy landing gear"},
		{"Computer, shields up.", "shields up"},
		{"Computer, open the star map.", "open star map"},
		{"Computer, flight ready.", "flight ready"},
	} {
		for _, voice := range []string{"alba", "george"} {
			if _, ok := m.Voices[voice]; !ok {
				continue
			}
			pcm, err := sp.Synthesize(c.line, voice)
			if err != nil {
				t.Fatal(err)
			}
			audio16 := Resample(pcm, sp.SampleRate(), SampleRate)
			got := make(chan []float32, 4)
			l := NewListener(vad, 2, func(p []float32, _ bool) { got <- p })
			// Half a second of quiet, the line, then a second of quiet, fed as a mic would.
			stream := append(append(make([]float32, SampleRate/2), audio16...), make([]float32, SampleRate)...)
			for i := 0; i < len(stream); i += 800 {
				l.Feed(stream[i:min(i+800, len(stream))])
			}
			var utt []float32
			select {
			case utt = <-got:
			case <-time.After(5 * time.Second):
				t.Fatalf("%s/%q: the listener found no utterance", voice, c.line)
			}
			t1 := time.Now()
			text, err := rec.Transcribe(utt)
			if err != nil {
				t.Fatal(err)
			}
			took := time.Since(t1)
			cmd, woke := SplitWake(text, "computer")
			best, score := Match(cmd, phrases)
			t.Logf("%-6s %-38q -> %-40q %5.2fs audio in %4dms -> %q (%.2f)", voice, c.line, text, float64(len(utt))/SampleRate, took.Milliseconds(), best, score)
			if !woke || best != c.want || score < 0.7 {
				t.Errorf("%s/%q: heard %q, woke=%v, matched %q (%.2f)", voice, c.line, text, woke, best, score)
			}
		}
	}
}
