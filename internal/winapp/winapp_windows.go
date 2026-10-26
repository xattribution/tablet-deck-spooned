//go:build windows

// Package winapp is Tablet Deck's Windows side: the small status window, the tray
// icon, one copy running at a time, Start with Windows and the tabletdeck:// link
// handler.
package winapp

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"fyne.io/systray"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

//go:embed icon.ico
var icon []byte

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// instanceID names the running app's mutex and show event, per settings folder.
func instanceID(dataDir string) string {
	id := `Local\TabletDeck-2`
	if dataDir != "" { // another settings folder is another deck
		h := uint32(2166136261)
		for _, c := range []byte(strings.ToLower(filepath.Clean(dataDir))) {
			h = (h ^ uint32(c)) * 16777619
		}
		id += fmt.Sprintf("-%08x", h)
	}
	return id
}

// SingleInstance returns false when Tablet Deck is already running for this user.
func SingleInstance(dataDir string) bool {
	name, _ := windows.UTF16PtrFromString(instanceID(dataDir))
	_, err := windows.CreateMutex(nil, false, name)
	return err != windows.ERROR_ALREADY_EXISTS
}

// ShowRunning asks the running app to show its window (starting the exe again does
// this). It reports whether a running app was found.
func ShowRunning(dataDir string) bool {
	name, _ := windows.UTF16PtrFromString(instanceID(dataDir) + "-show")
	ev, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, name)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(ev)
	// This process was started by the player, so it may bring a window to the front;
	// pass that on to the running app.
	procAllowSetForegroundWindow.Call(uintptr(0xFFFFFFFF)) // ASFW_ANY
	return windows.SetEvent(ev) == nil
}

// waitForShow shows the window whenever another start of the exe signals it.
func waitForShow(dataDir string, w *statusWindow) {
	name, _ := windows.UTF16PtrFromString(instanceID(dataDir) + "-show")
	ev, err := windows.CreateEvent(nil, 0, 0, name) // auto-reset
	if err != nil {
		return
	}
	for {
		if r, _ := windows.WaitForSingleObject(ev, windows.INFINITE); r != windows.WAIT_OBJECT_0 {
			return
		}
		w.show()
	}
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

// Run shows the status window (unless h.Hidden) and the tray icon, and blocks until
// the app quits from either. It must run on the main goroutine.
func Run(h Host) {
	w := startWindow(h)
	go waitForShow(h.DataDir, w)
	systray.SetOnTapped(w.show) // a click on the icon shows the window; the menu is on right-click
	systray.Run(func() {
		systray.SetIcon(icon)
		systray.SetTitle(h.Title)
		systray.SetTooltip(h.Title)
		show := systray.AddMenuItem("Show Tablet Deck", "Show the status window")
		release := systray.AddMenuItem("Release all keys", "Let go of any key Tablet Deck is holding")
		settings := systray.AddMenuItem("Open settings in browser", "The full settings page, for advanced use")
		startup := systray.AddMenuItemCheckbox("Start with Windows", "", StartsWithWindows())
		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "Release keys and stop Tablet Deck")
		go func() {
			for {
				select {
				case <-show.ClickedCh:
					w.show()
				case <-release.ClickedCh:
					h.ReleaseAll()
				case <-settings.ClickedCh:
					h.OpenSettings()
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
					w.hide()
					systray.Quit()
					return
				}
			}
		}()
	}, h.Quit)
}

// quitApp ends the app from the status window, the same way the tray's Quit does.
func quitApp() { systray.Quit() }
