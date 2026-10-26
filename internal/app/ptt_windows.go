//go:build windows

package app

import "golang.org/x/sys/windows"

var procGetAsyncKeyState = windows.NewLazySystemDLL("user32.dll").NewProc("GetAsyncKeyState")

// pttKeyDown reads only the push-to-talk key's up/down state: no keyboard hook, no
// other keys seen.
func pttKeyDown(vk int) bool {
	if vk <= 0 || vk > 255 {
		return false
	}
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return r&0x8000 != 0
}
