package app

import (
	"os"
	"testing"
	"time"

	"github.com/xattribution/tablet-deck-spooned/internal/config"
	"github.com/xattribution/tablet-deck-spooned/internal/speech"
)

// TestNativeVoiceCommand: "Computer, landing gear" spoken by Pocket TTS, heard by
// the gorelaxor engine, presses the gear key. Needs TD_MODELS (see the speech package).
func TestNativeVoiceCommand(t *testing.T) {
	dir := os.Getenv("TD_MODELS")
	if dir == "" {
		t.Skip("TD_MODELS not set")
	}
	a, f := startApp(t, "Jared")
	_ = a.Store.Update(func(c *config.Config) { c.ModelsDir = dir; c.Voice = true; c.Replies = "off" })
	a.voice.restart()
	deadline := time.Now().Add(60 * time.Second)
	for a.voice.Native() == nil {
		if time.Now().After(deadline) {
			t.Fatalf("native voice didn't start: %v", a.voice.status())
		}
		time.Sleep(100 * time.Millisecond)
	}
	n := a.voice.Native()
	models := a.FindSpeechModels()
	sp, err := speech.NewSpeaker(models.TTS, models.Voices, 2)
	if err != nil {
		t.Fatal(err)
	}
	say := func(line string) {
		pcm, err := sp.Synthesize(line, "george")
		if err != nil {
			t.Fatal(err)
		}
		n.utterance(speech.Resample(pcm, sp.SampleRate(), speech.SampleRate), false)
	}
	f.Events()
	say("Computer, landing gear.")
	if evs := f.Events(); len(evs) != 2 {
		t.Fatalf("landing gear: %d key events", len(evs))
	}
	say("Landing gear.") // no wake word: not for us
	if evs := f.Events(); len(evs) != 0 {
		t.Fatal("pressed without the wake word")
	}
	say("Computer, I'm AFK.")
	if !a.Afk() {
		t.Fatal("away mode didn't turn on")
	}
}
