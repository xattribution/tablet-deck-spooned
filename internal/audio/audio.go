// Package audio records the microphone and plays sound on this PC, in pure Go
// (Windows' WinMM wave API; no cgo, no DLLs to ship). Audio is float32 mono.
package audio

import "errors"

// ErrUnsupported is returned where there is no audio backend (off Windows).
var ErrUnsupported = errors.New("audio: not supported on this system")

// Device is an input or output device.
type Device struct {
	ID   int    `json:"id"` // -1 is the system default
	Name string `json:"name"`
}

// Recorder delivers microphone audio until Close.
type Recorder interface {
	Close() error
}

// Player plays audio one piece at a time.
type Player interface {
	// Write queues samples; it returns once they're queued, not played.
	Write(pcm []float32) error
	// Wait blocks until everything queued has played.
	Wait()
	// Stop drops whatever is queued and stops at once.
	Stop()
	Close() error
}

func toS16(pcm []float32, dst []byte) []byte {
	dst = dst[:0]
	for _, v := range pcm {
		if v > 1 {
			v = 1
		} else if v < -1 {
			v = -1
		}
		s := int16(v * 32767)
		dst = append(dst, byte(s), byte(uint16(s)>>8))
	}
	return dst
}

func fromS16(b []byte, dst []float32) []float32 {
	dst = dst[:0]
	for i := 0; i+1 < len(b); i += 2 {
		dst = append(dst, float32(int16(uint16(b[i])|uint16(b[i+1])<<8))/32768)
	}
	return dst
}
