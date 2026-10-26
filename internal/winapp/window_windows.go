//go:build windows

package winapp

import (
	"os"
	"runtime"
	"sync"
	"unsafe"

	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/sys/windows"
)

// The status window: a small, fixed-size native window that shows the app is running,
// the pairing code and its QR code, and closes the app with its close box. It runs on
// its own locked OS thread with its own message loop, next to the tray's.

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")

	procRegisterClassExW             = user32.NewProc("RegisterClassExW")
	procCreateWindowExW              = user32.NewProc("CreateWindowExW")
	procDefWindowProcW               = user32.NewProc("DefWindowProcW")
	procGetMessageW                  = user32.NewProc("GetMessageW")
	procTranslateMessage             = user32.NewProc("TranslateMessage")
	procDispatchMessageW             = user32.NewProc("DispatchMessageW")
	procPostMessageW                 = user32.NewProc("PostMessageW")
	procSendMessageW                 = user32.NewProc("SendMessageW")
	procShowWindow                   = user32.NewProc("ShowWindow")
	procSetForegroundWindow          = user32.NewProc("SetForegroundWindow")
	procAllowSetForegroundWindow     = user32.NewProc("AllowSetForegroundWindow")
	procIsWindowVisible              = user32.NewProc("IsWindowVisible")
	procIsIconic                     = user32.NewProc("IsIconic")
	procBeginPaint                   = user32.NewProc("BeginPaint")
	procEndPaint                     = user32.NewProc("EndPaint")
	procGetClientRect                = user32.NewProc("GetClientRect")
	procFillRect                     = user32.NewProc("FillRect")
	procDrawTextW                    = user32.NewProc("DrawTextW")
	procInvalidateRect               = user32.NewProc("InvalidateRect")
	procSetTimer                     = user32.NewProc("SetTimer")
	procSetWindowPos                 = user32.NewProc("SetWindowPos")
	procMoveWindow                   = user32.NewProc("MoveWindow")
	procGetDpiForWindow              = user32.NewProc("GetDpiForWindow")
	procAdjustWindowRectExForDpi     = user32.NewProc("AdjustWindowRectExForDpi")
	procAdjustWindowRectEx           = user32.NewProc("AdjustWindowRectEx")
	procLoadCursorW                  = user32.NewProc("LoadCursorW")
	procMonitorFromWindow            = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW              = user32.NewProc("GetMonitorInfoW")
	procCreateFontW                  = gdi32.NewProc("CreateFontW")
	procDeleteObject                 = gdi32.NewProc("DeleteObject")
	procSelectObject                 = gdi32.NewProc("SelectObject")
	procSetTextColor                 = gdi32.NewProc("SetTextColor")
	procSetBkMode                    = gdi32.NewProc("SetBkMode")
	procSetTextCharacterExtra        = gdi32.NewProc("SetTextCharacterExtra")
	procCreateSolidBrush             = gdi32.NewProc("CreateSolidBrush")
	procCreateCompatibleDC           = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap       = gdi32.NewProc("CreateCompatibleBitmap")
	procBitBlt                       = gdi32.NewProc("BitBlt")
	procDeleteDC                     = gdi32.NewProc("DeleteDC")
	procGetModuleHandleW             = kernel32.NewProc("GetModuleHandleW")
	procExtractIconExW               = shell32.NewProc("ExtractIconExW")
	procDwmSetWindowAttribute        = dwmapi.NewProc("DwmSetWindowAttribute")
	procSetThreadDpiAwarenessContext = user32.NewProc("SetThreadDpiAwarenessContext")
	procPostQuitMessage              = user32.NewProc("PostQuitMessage")
)

const (
	wsOverlapped  = 0x00000000
	wsCaption     = 0x00C00000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000
	wsChild       = 0x40000000
	wsVisible     = 0x10000000
	wsTabStop     = 0x00010000
	bsOwnerDraw   = 0x0000000B

	wmDestroy    = 0x0002
	wmPaint      = 0x000F
	wmClose      = 0x0010
	wmEraseBkgnd = 0x0014
	wmDrawItem   = 0x002B
	wmCommand    = 0x0111
	wmTimer      = 0x0113
	wmDpiChanged = 0x02E0
	wmApp        = 0x8000
	wmShowStatus = wmApp + 1
	wmHideStatus = wmApp + 2

	swHide    = 0
	swShow    = 5
	swRestore = 9

	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	dtLeft        = 0x0000
	dtCenter      = 0x0001
	dtVCenter     = 0x0004
	dtSingleLine  = 0x0020
	dtNoPrefix    = 0x0800
	dtEndEllipsis = 0x8000

	odsSelected = 0x0001

	idHide = 1
	idQuit = 2
	idCode = 3
)

// Colors (COLORREF is 0x00BBGGRR).
func rgb(r, g, b uint8) uintptr { return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16 }

var (
	colBg        = rgb(0x0A, 0x11, 0x1C)
	colText      = rgb(0xD7, 0xE3, 0xF4)
	colMuted     = rgb(0x7D, 0x8C, 0xA3)
	colAccent    = rgb(0xE8, 0xB5, 0x3D)
	colAccentHit = rgb(0xC9, 0x9A, 0x2C)
	colButton    = rgb(0x1A, 0x26, 0x36)
	colButtonHit = rgb(0x24, 0x34, 0x4A)
	colDark      = rgb(0x0A, 0x11, 0x1C)
	colQRLight   = rgb(0xF2, 0xF6, 0xFB)
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type point struct{ X, Y int32 }

type rect struct{ Left, Top, Right, Bottom int32 }

type winMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	Private uint32
}

type paintStruct struct {
	Hdc       uintptr
	Erase     int32
	Paint     rect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

type drawItemStruct struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   uintptr
	HDC        uintptr
	Item       rect
	ItemData   uintptr
}

type monitorInfo struct {
	Size    uint32
	Monitor rect
	Work    rect
	Flags   uint32
}

// statusWindow is the one status window.
type statusWindow struct {
	h     Host
	ready chan struct{}

	mu   sync.Mutex
	hwnd uintptr

	// Only touched on the window's thread.
	dpi     uint32
	fonts   map[string]uintptr
	buttons map[int]uintptr
	last    Status
	qr      [][]bool
	qrFor   string
}

var theWindow *statusWindow

// startWindow creates the window on its own thread and returns once it exists.
func startWindow(h Host) *statusWindow {
	w := &statusWindow{h: h, ready: make(chan struct{}), fonts: map[string]uintptr{}, buttons: map[int]uintptr{}}
	theWindow = w
	go w.loop()
	<-w.ready
	return w
}

func (w *statusWindow) handle() uintptr {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.hwnd
}

// show and hide may be called from any goroutine.
func (w *statusWindow) show() {
	if hw := w.handle(); hw != 0 {
		procPostMessageW.Call(hw, wmShowStatus, 0, 0)
	}
}

func (w *statusWindow) hide() {
	if hw := w.handle(); hw != 0 {
		procPostMessageW.Call(hw, wmHideStatus, 0, 0)
	}
}

func utf16(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

func (w *statusWindow) loop() {
	runtime.LockOSThread() // a window belongs to the thread that made it
	defer runtime.UnlockOSThread()
	// The exe's manifest makes the process per-monitor DPI aware; ask for it on this
	// thread too in case it runs without one (go run).
	if procSetThreadDpiAwarenessContext.Find() == nil {
		procSetThreadDpiAwarenessContext.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
	}
	inst, _, _ := procGetModuleHandleW.Call(0)
	cursor, _, _ := procLoadCursorW.Call(0, 32512) // IDC_ARROW
	var large, small uintptr
	if exe, err := os.Executable(); err == nil {
		procExtractIconExW.Call(uintptr(unsafe.Pointer(utf16(exe))), 0, uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&small)), 1)
	}
	bg, _, _ := procCreateSolidBrush.Call(colBg)
	cls := utf16("TabletDeckStatus")
	wc := wndClassEx{WndProc: windows.NewCallback(wndProc), Instance: inst, Icon: large, IconSm: small, Cursor: cursor,
		Background: bg, ClassName: cls}
	wc.Size = uint32(unsafe.Sizeof(wc))
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	const style = wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox
	hwnd, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(utf16(w.h.Title))), style,
		0x80000000, 0x80000000, 0, 0, 0, 0, inst, 0) // CW_USEDEFAULT
	w.mu.Lock()
	w.hwnd = hwnd
	w.mu.Unlock()
	close(w.ready)
	if hwnd == 0 {
		return
	}
	// A dark title bar to match (Windows 10 2004 and later; ignored elsewhere).
	on := int32(1)
	if procDwmSetWindowAttribute.Find() == nil {
		procDwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&on)), 4) // DWMWA_USE_IMMERSIVE_DARK_MODE
		capColor := uint32(colBg)
		procDwmSetWindowAttribute.Call(hwnd, 35, uintptr(unsafe.Pointer(&capColor)), 4) // DWMWA_CAPTION_COLOR (Windows 11)
	}

	w.dpi = dpiOf(hwnd)
	w.makeFonts()
	w.makeButtons(inst)
	w.sizeAndCenter()
	w.refresh(true)
	procSetTimer.Call(hwnd, 1, 1000, 0)
	if !w.h.Hidden {
		procShowWindow.Call(hwnd, swShow)
		procSetForegroundWindow.Call(hwnd)
	}

	var m winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func dpiOf(hwnd uintptr) uint32 {
	if procGetDpiForWindow.Find() == nil {
		if d, _, _ := procGetDpiForWindow.Call(hwnd); d != 0 {
			return uint32(d)
		}
	}
	return 96
}

// px scales a length at 96 DPI to the window's DPI.
func (w *statusWindow) px(n int) int32 { return int32((n*int(w.dpi) + 48) / 96) }

const (
	winW = 380 // client size at 96 DPI
	winH = 520
	pad  = 24
)

func (w *statusWindow) sizeAndCenter() {
	hwnd := w.hwnd
	r := rect{0, 0, w.px(winW), w.px(winH)}
	const style = wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox
	if procAdjustWindowRectExForDpi.Find() == nil {
		procAdjustWindowRectExForDpi.Call(uintptr(unsafe.Pointer(&r)), style, 0, 0, uintptr(w.dpi))
	} else {
		procAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&r)), style, 0, 0)
	}
	ww, wh := r.Right-r.Left, r.Bottom-r.Top
	mon, _, _ := procMonitorFromWindow.Call(hwnd, 2) // MONITOR_DEFAULTTONEAREST
	mi := monitorInfo{}
	mi.Size = uint32(unsafe.Sizeof(mi))
	x, y := int32(100), int32(100)
	if ok, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); ok != 0 {
		x = mi.Work.Left + (mi.Work.Right-mi.Work.Left-ww)/2
		y = mi.Work.Top + (mi.Work.Bottom-mi.Work.Top-wh)/2
	}
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(ww), uintptr(wh), swpNoZOrder|swpNoActivate)
}

// makeFonts (re)creates the fonts for the current DPI.
func (w *statusWindow) makeFonts() {
	for _, f := range w.fonts {
		procDeleteObject.Call(f)
	}
	mk := func(px int, face string) uintptr {
		f, _, _ := procCreateFontW.Call(uintptr(-w.px(px)), 0, 0, 0, 400, 0, 0, 0,
			1 /* DEFAULT_CHARSET */, 0, 0, 5 /* CLEARTYPE_QUALITY */, 0, uintptr(unsafe.Pointer(utf16(face))))
		return f
	}
	w.fonts = map[string]uintptr{
		"title":  mk(15, "Segoe UI Semibold"),
		"body":   mk(13, "Segoe UI"),
		"small":  mk(12, "Segoe UI"),
		"label":  mk(11, "Segoe UI Semibold"),
		"code":   mk(40, "Segoe UI Semibold"),
		"button": mk(13, "Segoe UI Semibold"),
	}
}

func (w *statusWindow) makeButtons(inst uintptr) {
	mk := func(id int, text string) {
		b, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(utf16("BUTTON"))), uintptr(unsafe.Pointer(utf16(text))),
			wsChild|wsVisible|wsTabStop|bsOwnerDraw, 0, 0, 0, 0, w.hwnd, uintptr(id), inst, 0)
		w.buttons[id] = b
	}
	mk(idHide, "Hide to tray")
	mk(idQuit, "Quit")
	if w.h.NewCode != nil {
		mk(idCode, "New code")
	}
	w.placeButtons()
}

// Where things go, at 96 DPI, top to bottom.
const (
	yTitle   = 22
	yStatus  = 50
	yGame    = 70
	yLabel   = 104
	yCode    = 120
	yQR      = 184
	qrSize   = 196
	yAddress = 392
	yNewCode = 412
	btnH     = 38
)

func (w *statusWindow) placeButtons() {
	gap := 12
	bw := (winW - 2*pad - gap) / 2
	by := winH - pad - btnH
	move := func(id, x, y, cw, ch int) {
		if b := w.buttons[id]; b != 0 {
			procMoveWindow.Call(b, uintptr(w.px(x)), uintptr(w.px(y)), uintptr(w.px(cw)), uintptr(w.px(ch)), 1)
		}
	}
	move(idHide, pad, by, bw, btnH)
	move(idQuit, pad+bw+gap, by, bw, btnH)
	move(idCode, (winW-90)/2, yNewCode, 90, 22)
}

// refresh reads the app's status and repaints when it changed.
func (w *statusWindow) refresh(force bool) {
	if w.h.Status == nil {
		return
	}
	if !force {
		if vis, _, _ := procIsWindowVisible.Call(w.hwnd); vis == 0 {
			return
		}
		if ic, _, _ := procIsIconic.Call(w.hwnd); ic != 0 {
			return
		}
	}
	s := w.h.Status()
	if !force && s == w.last {
		return
	}
	w.last = s
	if s.Link != w.qrFor {
		w.qr, w.qrFor = nil, s.Link
		if q, err := qrcode.New(s.Link, qrcode.Medium); err == nil && s.Link != "" {
			q.DisableBorder = true // the quiet zone is drawn below
			w.qr = q.Bitmap()
		}
	}
	procInvalidateRect.Call(w.hwnd, 0, 0)
}

func wndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	w := theWindow
	if w == nil || hwnd != w.handle() {
		r, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
		return r
	}
	switch msg {
	case wmPaint:
		w.paint()
		return 0
	case wmEraseBkgnd:
		return 1 // painted whole in WM_PAINT, without flicker
	case wmTimer:
		w.refresh(false)
		return 0
	case wmShowStatus:
		if ic, _, _ := procIsIconic.Call(hwnd); ic != 0 {
			procShowWindow.Call(hwnd, swRestore)
		} else {
			procShowWindow.Call(hwnd, swShow)
		}
		procSetForegroundWindow.Call(hwnd)
		w.refresh(true)
		return 0
	case wmHideStatus:
		procShowWindow.Call(hwnd, swHide)
		return 0
	case wmCommand:
		switch int(wparam & 0xFFFF) {
		case idHide:
			procShowWindow.Call(hwnd, swHide)
		case idQuit:
			procShowWindow.Call(hwnd, swHide)
			go quitApp()
		case idCode:
			if w.h.NewCode != nil {
				w.h.NewCode()
				w.refresh(true)
			}
		}
		return 0
	case wmDrawItem:
		w.drawButton(*(**drawItemStruct)(unsafe.Pointer(&lparam))) // lParam points at Windows-owned memory
		return 1
	case wmClose:
		// The close box quits, like Quit: release keys and stop the servers.
		procShowWindow.Call(hwnd, swHide)
		go quitApp()
		return 0
	case wmDpiChanged:
		w.dpi = uint32(wparam & 0xFFFF)
		r := *(**rect)(unsafe.Pointer(&lparam)) // the suggested window rectangle
		procSetWindowPos.Call(hwnd, 0, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), swpNoZOrder|swpNoActivate)
		w.makeFonts()
		w.placeButtons()
		procInvalidateRect.Call(hwnd, 0, 0)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0) // ends this thread's message loop
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}

// text draws one line in a box; spacing is extra space between letters (96 DPI px).
func (w *statusWindow) text(dc uintptr, s, font string, color uintptr, r rect, flags uintptr, spacing int) {
	procSelectObject.Call(dc, w.fonts[font])
	procSetTextColor.Call(dc, color)
	procSetTextCharacterExtra.Call(dc, uintptr(w.px(spacing)))
	p, _ := windows.UTF16FromString(s)
	procDrawTextW.Call(dc, uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)-1), uintptr(unsafe.Pointer(&r)),
		flags|dtSingleLine|dtVCenter|dtNoPrefix|dtEndEllipsis)
}

func fill(dc uintptr, r rect, color uintptr) {
	br, _, _ := procCreateSolidBrush.Call(color)
	procFillRect.Call(dc, uintptr(unsafe.Pointer(&r)), br)
	procDeleteObject.Call(br)
}

func (w *statusWindow) paint() {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(w.hwnd, uintptr(unsafe.Pointer(&ps)))
	defer procEndPaint.Call(w.hwnd, uintptr(unsafe.Pointer(&ps)))
	var cr rect
	procGetClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&cr)))
	cw, ch := cr.Right, cr.Bottom
	// Draw into a bitmap first, then copy it over in one go.
	dc, _, _ := procCreateCompatibleDC.Call(hdc)
	bmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(cw), uintptr(ch))
	old, _, _ := procSelectObject.Call(dc, bmp)
	defer func() {
		procBitBlt.Call(hdc, 0, 0, uintptr(cw), uintptr(ch), dc, 0, 0, 0x00CC0020) // SRCCOPY
		procSelectObject.Call(dc, old)
		procDeleteObject.Call(bmp)
		procDeleteDC.Call(dc)
	}()

	fill(dc, cr, colBg)
	procSetBkMode.Call(dc, 1) // TRANSPARENT
	s := w.last
	row := func(y, h int) rect { return rect{w.px(pad), w.px(y), cw - w.px(pad), w.px(y + h)} }

	w.text(dc, "TABLET DECK", "title", colAccent, row(yTitle, 22), dtLeft, 3)
	w.text(dc, s.Line(), "body", colText, row(yStatus, 18), dtLeft, 0)
	if s.Game != "" {
		w.text(dc, s.Game, "small", colMuted, row(yGame, 16), dtLeft, 0)
	}
	w.text(dc, "PAIRING CODE", "label", colMuted, row(yLabel, 16), dtCenter, 2)
	w.text(dc, s.Code, "code", colText, row(yCode, 52), dtCenter, 8)

	if n := len(w.qr); n > 0 {
		const quiet = 3 // light modules around the code, so phones can read it on a dark window
		mod := int(w.px(qrSize)) / (n + 2*quiet)
		if mod < 1 {
			mod = 1
		}
		size := int32(mod * (n + 2*quiet))
		x0 := (cw - size) / 2
		y0 := w.px(yQR) + (w.px(qrSize)-size)/2
		fill(dc, rect{x0, y0, x0 + size, y0 + size}, colQRLight)
		br, _, _ := procCreateSolidBrush.Call(colDark)
		m := int32(mod)
		for y, line := range w.qr {
			// One rectangle per run of dark modules.
			for x := 0; x < len(line); {
				if !line[x] {
					x++
					continue
				}
				end := x
				for end < len(line) && line[end] {
					end++
				}
				r := rect{x0 + int32(quiet+x)*m, y0 + int32(quiet+y)*m, x0 + int32(quiet+end)*m, y0 + int32(quiet+y+1)*m}
				procFillRect.Call(dc, uintptr(unsafe.Pointer(&r)), br)
				x = end
			}
		}
		procDeleteObject.Call(br)
	}
	w.text(dc, s.Address, "small", colMuted, row(yAddress, 16), dtCenter, 0)
}

func (w *statusWindow) drawButton(d *drawItemStruct) {
	pressed := d.ItemState&odsSelected != 0
	var bg, fg uintptr
	label, font := "", "button"
	switch d.CtlID {
	case idHide:
		bg, fg, label = colAccent, colDark, "Hide to tray"
		if pressed {
			bg = colAccentHit
		}
	case idQuit:
		bg, fg, label = colButton, colText, "Quit"
		if pressed {
			bg = colButtonHit
		}
	case idCode:
		bg, fg, label, font = colBg, colAccent, "New code", "small"
		if pressed {
			fg = colAccentHit
		}
	default:
		return
	}
	fill(d.HDC, d.Item, bg)
	procSetBkMode.Call(d.HDC, 1)
	w.text(d.HDC, label, font, fg, d.Item, dtCenter, 0)
}
