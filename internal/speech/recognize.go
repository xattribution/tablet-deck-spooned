package speech

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xattribution/gorelaxor/conformer"
	"github.com/xattribution/gorelaxor/whisper"
)

// SampleRate is what the recognizers take: 16 kHz mono.
const SampleRate = 16000

// Recognizer turns one utterance into text. Any model can sit behind it, so a
// new speech model is one more implementation.
type Recognizer interface {
	Name() string
	Transcribe(pcm []float32) (string, error)
	// SetHints gives words to expect (button names), for models that can use them.
	SetHints(words string)
	Close()
}

// LoadRecognizer loads a Parakeet (.gguf) or Whisper (ggml .bin) file.
func LoadRecognizer(path string, threads int) (Recognizer, error) {
	if strings.HasSuffix(strings.ToLower(path), ".bin") {
		m, err := whisper.Load(path)
		if err != nil {
			return nil, err
		}
		return &whisperRec{m: m, name: filepath.Base(path), threads: threads}, nil
	}
	m, err := conformer.Load(path)
	if err != nil {
		return nil, err
	}
	return &conformerRec{m: m, name: filepath.Base(path), threads: threads}, nil
}

type conformerRec struct {
	m       *conformer.Model
	name    string
	threads int
}

func (r *conformerRec) Name() string    { return r.name }
func (r *conformerRec) SetHints(string) {}
func (r *conformerRec) Close()          {}

func (r *conformerRec) Transcribe(pcm []float32) (string, error) {
	// An utterance is under the final pass's piece length, so no VAD is needed here.
	toks, err := r.m.Transcribe(pcm, nil, conformer.DefaultSegments, r.threads)
	if err != nil {
		return "", fmt.Errorf("%s: %w", r.name, err)
	}
	return strings.TrimSpace(r.m.TokenText(toks)), nil
}

type whisperRec struct {
	m       *whisper.Model
	name    string
	hints   string
	threads int
}

func (r *whisperRec) Name() string          { return r.name }
func (r *whisperRec) SetHints(words string) { r.hints = words }
func (r *whisperRec) Close()                { r.m.Close() }

func (r *whisperRec) Transcribe(pcm []float32) (string, error) {
	segs, err := r.m.Transcribe(context.Background(), pcm, whisper.Options{Threads: r.threads, Prompt: r.hints})
	if err != nil {
		return "", fmt.Errorf("%s: %w", r.name, err)
	}
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.Text)
		b.WriteByte(' ')
	}
	return strings.TrimSpace(b.String()), nil
}
