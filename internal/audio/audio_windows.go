//go:build windows

package audio

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winmm                 = windows.NewLazySystemDLL("winmm.dll")
	procWaveInGetNumDevs  = winmm.NewProc("waveInGetNumDevs")
	procWaveInGetDevCaps  = winmm.NewProc("waveInGetDevCapsW")
	procWaveInOpen        = winmm.NewProc("waveInOpen")
	procWaveInPrepare     = winmm.NewProc("waveInPrepareHeader")
	procWaveInUnprepare   = winmm.NewProc("waveInUnprepareHeader")
	procWaveInAddBuffer   = winmm.NewProc("waveInAddBuffer")
	procWaveInStart       = winmm.NewProc("waveInStart")
	procWaveInReset       = winmm.NewProc("waveInReset")
	procWaveInClose       = winmm.NewProc("waveInClose")
	procWaveOutGetNumDevs = winmm.NewProc("waveOutGetNumDevs")
	procWaveOutGetDevCaps = winmm.NewProc("waveOutGetDevCapsW")
	procWaveOutOpen       = winmm.NewProc("waveOutOpen")
	procWaveOutPrepare    = winmm.NewProc("waveOutPrepareHeader")
	procWaveOutUnprepare  = winmm.NewProc("waveOutUnprepareHeader")
	procWaveOutWrite      = winmm.NewProc("waveOutWrite")
	procWaveOutReset      = winmm.NewProc("waveOutReset")
	procWaveOutClose      = winmm.NewProc("waveOutClose")
)

const (
	waveMapper     = 0xFFFFFFFF
	callbackEvent  = 0x00050000
	whdrDone       = 0x00000001
	waveFormatPCM  = 1
	headerSize     = unsafe.Sizeof(waveHdr{})
	capsNameOffset = 8 // WAVEINCAPSW / WAVEOUTCAPSW: wMid, wPid, vDriverVersion, then szPname[32]
)

type waveFormat struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	Size           uint16
}

type waveHdr struct {
	Data          uintptr
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          uintptr
	Reserved      uintptr
}

func mmErr(what string, r uintptr) error {
	if r == 0 {
		return nil
	}
	return fmt.Errorf("audio: %s failed (MMRESULT %d)", what, r)
}

func pcmFormat(rate int) waveFormat {
	return waveFormat{FormatTag: waveFormatPCM, Channels: 1, SamplesPerSec: uint32(rate), AvgBytesPerSec: uint32(rate * 2), BlockAlign: 2, BitsPerSample: 16}
}

func devices(numDevs, getCaps *windows.LazyProc) []Device {
	out := []Device{{ID: -1, Name: "Windows default"}}
	n, _, _ := numDevs.Call()
	for i := uintptr(0); i < n; i++ {
		var caps [8 + 64 + 64]byte
		if r, _, _ := getCaps.Call(i, uintptr(unsafe.Pointer(&caps[0])), uintptr(len(caps))); r != 0 {
			continue
		}
		name := windows.UTF16PtrToString((*uint16)(unsafe.Pointer(&caps[capsNameOffset])))
		out = append(out, Device{ID: int(i), Name: name})
	}
	return out
}

// Inputs lists microphones.
func Inputs() []Device { return devices(procWaveInGetNumDevs, procWaveInGetDevCaps) }

// Outputs lists speakers and headsets.
func Outputs() []Device { return devices(procWaveOutGetNumDevs, procWaveOutGetDevCaps) }

func devID(id int) uintptr {
	if id < 0 {
		return waveMapper
	}
	return uintptr(id)
}

// ---------- recording ----------

type recorder struct {
	h     uintptr
	event windows.Handle
	// WinMM keeps pointers to these after each call. Go's collector never moves
	// memory, and the recorder holds them until Close, so they stay put.
	hdrs []waveHdr
	bufs [][]byte
	n    int
	stop chan struct{}
	done chan struct{}
}

// Record opens a microphone (id -1: the default) at rate Hz and calls fn with each
// chunk (chunkMs long) on its own goroutine until Close.
func Record(id, rate, chunkMs int, fn func(pcm []float32)) (Recorder, error) {
	ev, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return nil, err
	}
	r := &recorder{event: ev, n: 8, stop: make(chan struct{}), done: make(chan struct{})}
	format := pcmFormat(rate)
	if e := mmErr("waveInOpen", call(procWaveInOpen, uintptr(unsafe.Pointer(&r.h)), devID(id), uintptr(unsafe.Pointer(&format)), uintptr(ev), 0, callbackEvent)); e != nil {
		windows.CloseHandle(ev)
		return nil, e
	}
	bufBytes := rate * 2 * chunkMs / 1000
	r.hdrs = make([]waveHdr, r.n)
	r.bufs = make([][]byte, r.n)
	for i := 0; i < r.n; i++ {
		r.bufs[i] = make([]byte, bufBytes)
		h := r.hdr(i)
		*h = waveHdr{Data: uintptr(unsafe.Pointer(&r.bufs[i][0])), BufferLength: uint32(bufBytes)}
		call(procWaveInPrepare, r.h, uintptr(unsafe.Pointer(h)), headerSize)
		call(procWaveInAddBuffer, r.h, uintptr(unsafe.Pointer(h)), headerSize)
	}
	if e := mmErr("waveInStart", call(procWaveInStart, r.h)); e != nil {
		r.Close()
		return nil, e
	}
	go r.loop(fn)
	return r, nil
}

func call(p *windows.LazyProc, a ...uintptr) uintptr {
	r, _, _ := p.Call(a...)
	return r
}

func (r *recorder) hdr(i int) *waveHdr { return &r.hdrs[i] }

func (r *recorder) loop(fn func([]float32)) {
	defer close(r.done)
	var pcm []float32
	next := 0 // buffers come back in the order they were added
	for {
		select {
		case <-r.stop:
			return
		default:
		}
		_, _ = windows.WaitForSingleObject(r.event, 200)
		for {
			h := r.hdr(next)
			if h.Flags&whdrDone == 0 {
				break
			}
			b := r.bufs[next][:h.BytesRecorded]
			pcm = fromS16(b, pcm)
			if len(pcm) > 0 {
				fn(append([]float32(nil), pcm...))
			}
			h.Flags &^= whdrDone
			select {
			case <-r.stop:
				return
			default:
			}
			call(procWaveInAddBuffer, r.h, uintptr(unsafe.Pointer(h)), headerSize)
			next = (next + 1) % r.n
		}
	}
}

func (r *recorder) closeHandle() {
	call(procWaveInClose, r.h)
	windows.CloseHandle(r.event)
}

func (r *recorder) Close() error {
	select {
	case <-r.stop:
		return nil
	default:
	}
	close(r.stop)
	windows.SetEvent(r.event)
	<-r.done
	call(procWaveInReset, r.h)
	for i := 0; i < r.n; i++ {
		call(procWaveInUnprepare, r.h, uintptr(unsafe.Pointer(r.hdr(i))), headerSize)
	}
	r.closeHandle()
	return nil
}

// ---------- playing ----------

type player struct {
	mu    sync.Mutex
	h     uintptr
	event windows.Handle
	hdrs  []waveHdr // kept until Close; see recorder
	bufs  [][]byte
	n     int
	size  int // bytes per buffer
	busy  []bool
	tmp   []byte
}

// NewPlayer opens a speaker (id -1: the default) for mono audio at rate Hz.
func NewPlayer(id, rate int) (Player, error) {
	ev, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return nil, err
	}
	p := &player{event: ev, n: 16, size: rate * 2 / 5} // 200 ms buffers
	format := pcmFormat(rate)
	if e := mmErr("waveOutOpen", call(procWaveOutOpen, uintptr(unsafe.Pointer(&p.h)), devID(id), uintptr(unsafe.Pointer(&format)), uintptr(ev), 0, callbackEvent)); e != nil {
		windows.CloseHandle(ev)
		return nil, e
	}
	p.hdrs = make([]waveHdr, p.n)
	p.bufs = make([][]byte, p.n)
	for i := range p.bufs {
		p.bufs[i] = make([]byte, p.size)
	}
	p.busy = make([]bool, p.n)
	return p, nil
}

func (p *player) hdr(i int) *waveHdr { return &p.hdrs[i] }

// reap frees buffers that finished playing.
func (p *player) reap() {
	for i := 0; i < p.n; i++ {
		if h := p.hdr(i); p.busy[i] && h.Flags&whdrDone != 0 {
			call(procWaveOutUnprepare, p.h, uintptr(unsafe.Pointer(h)), headerSize)
			p.busy[i] = false
		}
	}
}

func (p *player) Write(pcm []float32) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tmp = toS16(pcm, p.tmp)
	data := p.tmp
	for len(data) > 0 {
		p.reap()
		free := -1
		for i, b := range p.busy {
			if !b {
				free = i
				break
			}
		}
		if free < 0 {
			p.mu.Unlock()
			_, _ = windows.WaitForSingleObject(p.event, 100)
			p.mu.Lock()
			continue
		}
		n := copy(p.bufs[free], data)
		h := p.hdr(free)
		*h = waveHdr{Data: uintptr(unsafe.Pointer(&p.bufs[free][0])), BufferLength: uint32(n)}
		call(procWaveOutPrepare, p.h, uintptr(unsafe.Pointer(h)), headerSize)
		if e := mmErr("waveOutWrite", call(procWaveOutWrite, p.h, uintptr(unsafe.Pointer(h)), headerSize)); e != nil {
			call(procWaveOutUnprepare, p.h, uintptr(unsafe.Pointer(h)), headerSize)
			return e
		}
		p.busy[free] = true
		data = data[n:]
	}
	return nil
}

func (p *player) Wait() {
	for {
		p.mu.Lock()
		p.reap()
		left := false
		for _, b := range p.busy {
			left = left || b
		}
		p.mu.Unlock()
		if !left {
			return
		}
		_, _ = windows.WaitForSingleObject(p.event, 50)
		time.Sleep(time.Millisecond)
	}
}

func (p *player) Stop() {
	p.mu.Lock()
	call(procWaveOutReset, p.h)
	p.reap()
	p.mu.Unlock()
}

func (p *player) Close() error {
	p.Stop()
	call(procWaveOutClose, p.h)
	windows.CloseHandle(p.event)
	return nil
}
