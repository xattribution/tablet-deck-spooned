package speech

import (
	"math"
	"sync"

	"github.com/xattribution/gorelaxor/conformer"
)

// Listener cuts a microphone stream into utterances: MarbleNet marks speech every
// 20 ms, and an utterance ends after a short pause. While push-to-talk is held it
// records everything instead, and sends it all when let go.
type Listener struct {
	vad     *conformer.VAD // nil: a plain loudness gate
	threads int
	emit    func(pcm []float32, ptt bool)

	mu        sync.Mutex
	buf       []float32 // recent audio, starting at sample bufStart
	bufStart  int
	pos       int // samples seen
	lastCheck int
	run       int // speech decisions in a row
	active    bool
	start     int
	lastSpeak int
	ptt       bool
	pttStart  int
	muted     bool
	level     float32 // recent loudness, for a level meter
}

// Endpointing settings, in samples at 16 kHz.
const (
	stepSamples   = SampleRate / 10       // decide every 100 ms
	windowSamples = SampleRate            // over the last second
	lagSamples    = SampleRate / 5        // judged 200 ms back, away from the window's edge
	preRoll       = SampleRate * 35 / 100 // keep 350 ms before speech started
	tailPad       = SampleRate / 4        // and 250 ms after it stopped
	endSilence    = SampleRate * 55 / 100 // an utterance ends after this much quiet
	maxUtterance  = SampleRate * 8
	keepSamples   = SampleRate * 12
)

// NewListener returns a listener; emit runs on its own goroutine, one utterance at a time.
func NewListener(vad *conformer.VAD, threads int, emit func(pcm []float32, ptt bool)) *Listener {
	l := &Listener{vad: vad, threads: threads}
	q := make(chan func(), 4)
	go func() {
		for job := range q {
			job()
		}
	}()
	l.emit = func(pcm []float32, ptt bool) {
		select {
		case q <- func() { emit(pcm, ptt) }:
		default: // still busy with earlier speech: drop rather than fall behind
		}
	}
	return l
}

// Level is the recent loudness, 0 to 1.
func (l *Listener) Level() float32 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

// Mute stops listening (while Tablet Deck is talking, so it doesn't hear itself).
func (l *Listener) Mute(on bool) {
	l.mu.Lock()
	l.muted = on
	if on {
		l.active, l.run = false, 0
	}
	l.mu.Unlock()
}

// SetPTT starts or ends push-to-talk.
func (l *Listener) SetPTT(on bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if on == l.ptt {
		return
	}
	l.ptt = on
	if on {
		l.active, l.run = false, 0
		l.pttStart = max(l.bufStart, l.pos-SampleRate/5)
		return
	}
	if l.pos-l.pttStart > SampleRate/4 {
		l.emit(l.slice(l.pttStart, l.pos), true)
	}
}

func (l *Listener) slice(from, to int) []float32 {
	from = max(from, l.bufStart)
	to = min(to, l.pos)
	if to <= from {
		return nil
	}
	return append([]float32(nil), l.buf[from-l.bufStart:to-l.bufStart]...)
}

// Feed takes microphone audio, 16 kHz mono, in any size of chunk.
func (l *Listener) Feed(pcm []float32) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var sum float64
	for _, v := range pcm {
		sum += float64(v * v)
	}
	if len(pcm) > 0 {
		l.level = float32(math.Min(1, math.Sqrt(sum/float64(len(pcm)))*6))
	}
	if l.muted {
		l.bufStart, l.buf, l.pos = l.pos+len(pcm), l.buf[:0], l.pos+len(pcm)
		l.lastCheck = l.pos
		return
	}
	l.buf = append(l.buf, pcm...)
	l.pos += len(pcm)
	if l.ptt {
		if l.pos-l.pttStart > 2*maxUtterance { // a stuck key: send what there is
			l.emit(l.slice(l.pttStart, l.pos), true)
			l.pttStart = l.pos
		}
	} else {
		for l.pos-l.lastCheck >= stepSamples {
			l.lastCheck += stepSamples
			l.decide(l.lastCheck)
		}
	}
	// Keep the buffer bounded, but never drop audio an utterance still needs.
	keepFrom := l.pos - keepSamples
	if l.active {
		keepFrom = min(keepFrom, l.start)
	}
	if l.ptt {
		keepFrom = min(keepFrom, l.pttStart)
	}
	if drop := keepFrom - l.bufStart; drop > SampleRate {
		l.buf = append(l.buf[:0], l.buf[drop:]...)
		l.bufStart += drop
	}
}

// decide judges the 100 ms ending lagSamples before at and moves the utterance on.
func (l *Listener) decide(at int) {
	end := at - lagSamples
	if end-l.bufStart < stepSamples {
		return
	}
	speech := l.isSpeech(end)
	switch {
	case !l.active && speech:
		l.run++
		if l.run >= 2 { // 200 ms of speech
			l.active, l.start, l.lastSpeak = true, end-2*stepSamples-preRoll, end
		}
	case !l.active:
		l.run = 0
	case speech:
		l.lastSpeak = end
	}
	if !l.active {
		return
	}
	if end-l.lastSpeak >= endSilence || end-l.start >= maxUtterance {
		l.emit(l.slice(l.start, l.lastSpeak+tailPad), false)
		l.active, l.run = false, 0
	}
}

// isSpeech: are most of the 20 ms frames in [end-100ms, end) speech?
func (l *Listener) isSpeech(end int) bool {
	from := max(l.bufStart, end-windowSamples)
	win := l.buf[from-l.bufStart : end-l.bufStart]
	if l.vad == nil {
		var sum float64
		for _, v := range win[len(win)-stepSamples:] {
			sum += float64(v * v)
		}
		return math.Sqrt(sum/stepSamples) > 0.02
	}
	probs, err := l.vad.Probs(win, l.threads)
	if err != nil || len(probs) < 5 {
		return false
	}
	n := 0
	for _, p := range probs[len(probs)-5:] {
		if p >= 0.5 {
			n++
		}
	}
	return n >= 3
}
