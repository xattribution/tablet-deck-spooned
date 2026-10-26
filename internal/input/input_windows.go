//go:build windows

package input

import (
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/xattribution/tablet-deck-spooned/internal/keys"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procSendInput                = user32.NewProc("SendInput")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procShowWindow               = user32.NewProc("ShowWindow")
	procIsIconic                 = user32.NewProc("IsIconic")
	procIsWindow                 = user32.NewProc("IsWindow")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
	procGetWindow                = user32.NewProc("GetWindow")
	kernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	procGetCurrentThreadId       = kernel32.NewProc("GetCurrentThreadId")
)

const (
	inputKeyboard       = 1
	keyeventfExtended   = 0x0001
	keyeventfKeyUp      = 0x0002
	keyeventfScancode   = 0x0008
	swRestore           = 9
	gwOwner             = 4
	vkUnassigned        = 0xE8 // an unassigned virtual key: harmless to press
	processQueryLimited = 0x1000
)

// keyboardInput is INPUT with its KEYBDINPUT member, padded to the size of the union's
// largest member (MOUSEINPUT) so SendInput accepts cbSize on 64-bit Windows.
type keyboardInput struct {
	typ       uint32
	_         uint32
	wVk       uint16
	wScan     uint16
	dwFlags   uint32
	time      uint32
	_         uint32
	extraInfo uintptr
	_         [8]byte
}

type winDriver struct {
	dry bool

	mu      sync.Mutex
	games   []string
	names   map[uint32]string // pid -> lowercased exe base name, for foreground checks
	paths   map[uint32]string // pid -> full exe path
	namesAt time.Time
	hwnd    uintptr // the game's main window, cached
	pid     uint32
}

// New returns the Windows driver. With dry set it never presses a key, but still
// reports the game's focus, so a dry run can say what would have been blocked.
func New(dry bool) Driver {
	return &winDriver{dry: dry, games: NormalizeGames(nil), names: map[uint32]string{}, paths: map[uint32]string{}}
}

func (d *winDriver) Name() string {
	if d.dry {
		return "windows (dry run)"
	}
	return "windows"
}

func (d *winDriver) SetGames(names []string) {
	d.mu.Lock()
	d.games = NormalizeGames(names)
	d.hwnd, d.pid = 0, 0
	d.mu.Unlock()
}

func (d *winDriver) Key(sc keys.Scan, down bool) error {
	if d.dry {
		return nil
	}
	flags := uint32(keyeventfScancode)
	if sc.Ext {
		flags |= keyeventfExtended
	}
	if !down {
		flags |= keyeventfKeyUp
	}
	in := keyboardInput{typ: inputKeyboard, wScan: sc.Code, dwFlags: flags}
	n, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	if n != 1 {
		return err
	}
	return nil
}

func (d *winDriver) sendVK(vk uint16, up bool) {
	flags := uint32(0)
	if up {
		flags = keyeventfKeyUp
	}
	in := keyboardInput{typ: inputKeyboard, wVk: vk, dwFlags: flags}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

func windowPID(hwnd uintptr) (pid uint32, thread uint32) {
	t, _, _ := procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid, uint32(t)
}

// exeOf returns the lowercased exe name and full path of a process, cached briefly so
// a press costs microseconds, not a process open.
func (d *winDriver) exeOf(pid uint32) (string, string) {
	if pid == 0 {
		return "", ""
	}
	if time.Since(d.namesAt) > 30*time.Second {
		d.names, d.paths, d.namesAt = map[uint32]string{}, map[uint32]string{}, time.Now()
	}
	if n, ok := d.names[pid]; ok {
		return n, d.paths[pid]
	}
	full := ""
	if h, err := windows.OpenProcess(processQueryLimited, false, pid); err == nil {
		buf := make([]uint16, windows.MAX_LONG_PATH)
		size := uint32(len(buf))
		if windows.QueryFullProcessImageName(h, 0, &buf[0], &size) == nil {
			full = windows.UTF16ToString(buf[:size])
		}
		windows.CloseHandle(h)
	}
	name := strings.ToLower(filepath.Base(full))
	if full == "" {
		name = ""
	}
	d.names[pid], d.paths[pid] = name, full
	return name, full
}

func (d *winDriver) isGame(name string) bool {
	for _, g := range d.games {
		if name == g {
			return true
		}
	}
	return false
}

func (d *winDriver) Game() Game {
	d.mu.Lock()
	defer d.mu.Unlock()
	fg, _, _ := procGetForegroundWindow.Call()
	fpid, _ := windowPID(fg)
	front, frontPath := d.exeOf(fpid)
	if d.isGame(front) {
		d.hwnd, d.pid = fg, fpid
		return Game{Running: true, Focused: true, PID: fpid, Exe: frontPath, Front: front}
	}
	h := d.findGameWindow()
	if h == 0 {
		return Game{Front: front}
	}
	_, path := d.exeOf(d.pid)
	return Game{Running: true, PID: d.pid, Exe: path, Front: front}
}

// findGameWindow returns the game's main window: the cached one while it's still
// there, else the first visible, unowned top-level window of a game process.
func (d *winDriver) findGameWindow() uintptr {
	if d.hwnd != 0 {
		if ok, _, _ := procIsWindow.Call(d.hwnd); ok != 0 {
			if pid, _ := windowPID(d.hwnd); pid == d.pid {
				return d.hwnd
			}
		}
		d.hwnd, d.pid = 0, 0
	}
	// One callback for the life of the process: Windows callbacks are never freed.
	enumOnce.Do(func() { enumCB = windows.NewCallback(enumWindow) })
	enumMu.Lock()
	enumFor, enumFound, enumPID = d, 0, 0
	procEnumWindows.Call(enumCB, 0)
	found, foundPID := enumFound, enumPID
	enumFor = nil
	enumMu.Unlock()
	d.hwnd, d.pid = found, foundPID
	return found
}

// EnumWindows state; only used with the driver's lock held.
var (
	enumMu    sync.Mutex
	enumOnce  sync.Once
	enumCB    uintptr
	enumFor   *winDriver
	enumFound uintptr
	enumPID   uint32
)

func enumWindow(h uintptr, _ uintptr) uintptr {
	if v, _, _ := procIsWindowVisible.Call(h); v == 0 {
		return 1
	}
	if owner, _, _ := procGetWindow.Call(h, gwOwner); owner != 0 {
		return 1
	}
	pid, _ := windowPID(h)
	if name, _ := enumFor.exeOf(pid); enumFor.isGame(name) {
		enumFound, enumPID = h, pid
		return 0
	}
	return 1
}

// BringToFront puts the game in front. Windows only lets the process that had the last
// input change the foreground window, so it attaches to the current foreground
// window's input queue and, if that isn't enough, presses an unassigned key first so
// the last input is ours.
func (d *winDriver) BringToFront(timeout time.Duration) bool {
	d.mu.Lock()
	h := d.findGameWindow()
	d.mu.Unlock()
	if h == 0 {
		return false
	}
	if ic, _, _ := procIsIconic.Call(h); ic != 0 {
		procShowWindow.Call(h, swRestore)
	}
	front := func() bool { fg, _, _ := procGetForegroundWindow.Call(); return fg == h }
	if front() {
		return true
	}
	fg, _, _ := procGetForegroundWindow.Call()
	_, fgThread := windowPID(fg)
	me, _, _ := procGetCurrentThreadId.Call()
	if fgThread != 0 && uint32(me) != fgThread {
		procAttachThreadInput.Call(me, uintptr(fgThread), 1)
		procSetForegroundWindow.Call(h)
		procBringWindowToTop.Call(h)
		procAttachThreadInput.Call(me, uintptr(fgThread), 0)
	} else {
		procSetForegroundWindow.Call(h)
	}
	if !front() {
		d.sendVK(vkUnassigned, false)
		d.sendVK(vkUnassigned, true)
		procSetForegroundWindow.Call(h)
	}
	deadline := time.Now().Add(timeout)
	for !front() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
	return true
}
