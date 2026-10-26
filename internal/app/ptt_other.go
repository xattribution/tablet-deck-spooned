//go:build !windows

package app

func pttKeyDown(int) bool { return false }
