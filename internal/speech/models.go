// Package speech is Tablet Deck's own voice: it hears commands with gorelaxor's
// speech recognition (Parakeet or Whisper, cut into utterances by MarbleNet) and
// answers with Pocket TTS, all on this PC, in pure Go.
package speech

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Models are the model files found in the models folders.
type Models struct {
	Dirs   []string          `json:"dirs"`
	STT    []string          `json:"stt"`    // speech to text, best first
	VAD    string            `json:"vad"`    // MarbleNet frame VAD
	TTS    string            `json:"tts"`    // Pocket TTS model
	Voices map[string]string `json:"voices"` // voice name -> file
}

// sttRank orders speech-to-text files: Parakeet v2 (English, most accurate and
// fastest), v3, then Whisper from small to large (bigger is slower to answer).
func sttRank(name string) int {
	n := strings.ToLower(name)
	switch {
	case strings.HasPrefix(n, "parakeet-tdt-0.6b-v2"):
		return 0
	case strings.HasPrefix(n, "parakeet-tdt-0.6b-v3"):
		return 1
	case strings.HasPrefix(n, "ggml-base.en"):
		return 2
	case strings.HasPrefix(n, "ggml-small.en"):
		return 3
	case strings.HasPrefix(n, "ggml-tiny.en"):
		return 4
	case strings.HasPrefix(n, "ggml-large-v3-turbo"):
		return 5
	case strings.HasPrefix(n, "ggml-") && strings.HasSuffix(n, ".bin"):
		return 6
	}
	return -1
}

// FindModels looks through the folders (and two levels of subfolders, so
// "Speech-to-text" and "text-to-speech" folders work) for model files.
func FindModels(dirs ...string) Models {
	m := Models{Voices: map[string]string{}}
	var stt []string
	seen := map[string]bool{}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			continue
		}
		m.Dirs = append(m.Dirs, d)
		base := strings.Count(filepath.Clean(d), string(os.PathSeparator))
		_ = filepath.WalkDir(d, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if e.IsDir() {
				if strings.Count(filepath.Clean(p), string(os.PathSeparator))-base > 2 {
					return filepath.SkipDir
				}
				return nil
			}
			name := e.Name()
			low := strings.ToLower(name)
			if seen[low] {
				return nil
			}
			seen[low] = true
			switch {
			case sttRank(name) >= 0:
				stt = append(stt, p)
			case strings.Contains(low, "marblenet") && strings.HasSuffix(low, ".gguf"):
				m.VAD = p
			case strings.HasPrefix(low, "pocket-tts") && strings.HasSuffix(low, ".gguf"):
				// Q8_0 over F32: a quarter of the memory, the same voice.
				if m.TTS == "" || strings.Contains(low, "q8_0") {
					m.TTS = p
				}
			case strings.HasPrefix(low, "voice-") && strings.HasSuffix(low, ".gguf"):
				m.Voices[strings.TrimSuffix(strings.TrimPrefix(low, "voice-"), ".gguf")] = p
			}
			return nil
		})
	}
	sort.SliceStable(stt, func(i, j int) bool { return sttRank(filepath.Base(stt[i])) < sttRank(filepath.Base(stt[j])) })
	m.STT = stt
	return m
}

// VoiceNames lists the voices, sorted.
func (m Models) VoiceNames() []string {
	out := make([]string, 0, len(m.Voices))
	for n := range m.Voices {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// PickSTT returns the file to use: want (a file name or path) when found, else the best.
func (m Models) PickSTT(want string) string {
	if want != "" && want != "auto" {
		for _, p := range m.STT {
			if strings.EqualFold(filepath.Base(p), filepath.Base(want)) || p == want {
				return p
			}
		}
	}
	if len(m.STT) > 0 {
		return m.STT[0]
	}
	return ""
}
