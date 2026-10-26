//go:build windows

// Package winapp is Tablet Deck's Windows side: the tray icon, one copy running at a
// time, Start with Windows, the tabletdeck:// link handler, and opening the control
// panel and the touch deck in their own windows.
package winapp

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"fyne.io/systray"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

//go:embed icon.ico
var icon []byte

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// SingleInstance returns false when Tablet Deck is already running for this user.
func SingleInstance() bool {
	name, _ := windows.UTF16PtrFromString(`Local\TabletDeck-2`)
	_, err := windows.CreateMutex(nil, false, name)
	return err != windows.ERROR_ALREADY_EXISTS
}

// AttachConsole shows output when started from a command prompt (the app is built as
// a windowed program, so it has no console of its own).
func AttachConsole() {
	k := windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")
	if r, _, _ := k.Call(uintptr(0xFFFFFFFF)); r == 0 { // ATTACH_PARENT_PROCESS
		return
	}
	if h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil && h != 0 {
		os.Stdout = os.NewFile(uintptr(h), "stdout")
	}
	if h, err := windows.GetStdHandle(windows.STD_ERROR_HANDLE); err == nil && h != 0 {
		os.Stderr = os.NewFile(uintptr(h), "stderr")
	}
}

// StartsWithWindows reports whether Tablet Deck starts at sign-in.
func StartsWithWindows() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue("TabletDeck")
	return err == nil
}

// SetStartWithWindows turns starting at sign-in on or off.
func SetStartWithWindows(on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		err := k.DeleteValue("TabletDeck")
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	exe, _ := os.Executable()
	return k.SetStringValue("TabletDeck", fmt.Sprintf(`"%s" --tray`, exe))
}

// RegisterLinks makes tabletdeck:// links (Stream Deck "Website" actions, shortcuts,
// other apps) run Tablet Deck. Per user, no admin rights needed.
func RegisterLinks() error {
	exe, _ := os.Executable()
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\tabletdeck`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	_ = k.SetStringValue("", "URL:Tablet Deck")
	_ = k.SetStringValue("URL Protocol", "")
	k.Close()
	c, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\tabletdeck\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer c.Close()
	return c.SetStringValue("", fmt.Sprintf(`"%s" url "%%1"`, exe))
}

// edge finds Microsoft Edge, which opens pages as app windows without browser chrome.
func edge() string {
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LOCALAPPDATA"} {
		p := filepath.Join(os.Getenv(env), "Microsoft", "Edge", "Application", "msedge.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// OpenWindow opens a page as its own app window (Edge), else in the default browser.
// profile keeps the window's size and place apart from the player's browsing.
func OpenWindow(url, profile string, w, h int) {
	if e := edge(); e != "" {
		dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "TabletDeck", "windows", profile)
		cmd := exec.Command(e, "--app="+url, fmt.Sprintf("--window-size=%d,%d", w, h), "--user-data-dir="+dir,
			"--no-first-run", "--disable-features=Translate")
		if cmd.Start() == nil {
			return
		}
	}
	OpenBrowser(url)
}

// OpenBrowser opens a page in the default browser.
func OpenBrowser(url string) {
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}

// Message shows a message box (for errors before anything else can show them).
func Message(title, text string) {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(text)
	windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x10)
}

// Tray is what the tray menu can do.
type Tray struct {
	Title        string
	OpenPanel    func()
	OpenPairing  func()
	OpenTouch    func()
	ReleaseAll   func()
	ToggleAfk    func() bool // returns the new state
	Quit         func()
}

// RunTray shows the tray icon and blocks until Quit. It must run on the main thread.
func RunTray(t Tray) {
	systray.Run(func() {
		systray.SetIcon(icon)
		systray.SetTitle(t.Title)
		systray.SetTooltip(t.Title)
		open := systray.AddMenuItem("Open Tablet Deck", "Control panel")
		pair := systray.AddMenuItem("Pair a tablet or phone…", "Show the QR code")
		touch := systray.AddMenuItem("Touch deck on this PC", "Open the deck in a window for a touchscreen")
		systray.AddSeparator()
		afk := systray.AddMenuItemCheckbox("Away (AFK)", "Opens AFK-only share links for your crew", false)
		release := systray.AddMenuItem("Release all keys", "Let go of any key Tablet Deck is holding")
		startup := systray.AddMenuItemCheckbox("Start with Windows", "", StartsWithWindows())
		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "")
		go func() {
			for {
				select {
				case <-open.ClickedCh:
					t.OpenPanel()
				case <-pair.ClickedCh:
					t.OpenPairing()
				case <-touch.ClickedCh:
					t.OpenTouch()
				case <-afk.ClickedCh:
					if t.ToggleAfk() {
						afk.Check()
					} else {
						afk.Uncheck()
					}
				case <-release.ClickedCh:
					t.ReleaseAll()
				case <-startup.ClickedCh:
					on := !StartsWithWindows()
					if SetStartWithWindows(on) == nil {
						if on {
							startup.Check()
						} else {
							startup.Uncheck()
						}
					}
				case <-quit.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, t.Quit)
}
