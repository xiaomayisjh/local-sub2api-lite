//go:build debug && windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

// ensureConsole guarantees a stdout/stderr console exists for the debug build,
// even when launched by double-clicking. We try AttachConsole(parent) first (so
// the binary cooperates with `local-sub2api-lite-debug.exe` launched from a
// terminal), and fall back to AllocConsole when no parent console exists.
func init() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	attachConsole := kernel32.NewProc("AttachConsole")
	allocConsole := kernel32.NewProc("AllocConsole")
	getStdHandle := kernel32.NewProc("GetStdHandle")
	setStdHandle := kernel32.NewProc("SetStdHandle")
	const attachParentProcess = ^uintptr(0) // -1 / DWORD(-1)
	const stdOutputHandle = ^uintptr(10) + 1 // -11
	const stdErrorHandle = ^uintptr(11) + 1  // -12

	r1, _, _ := attachConsole.Call(attachParentProcess)
	if r1 == 0 {
		r1, _, _ = allocConsole.Call()
		if r1 == 0 {
			return
		}
	}

	stdoutHandle, _, _ := getStdHandle.Call(stdOutputHandle)
	stderrHandle, _, _ := getStdHandle.Call(stdErrorHandle)
	_ = setStdHandle
	_ = unsafe.Pointer(nil)

	if stdoutHandle != 0 {
		if f := os.NewFile(uintptr(stdoutHandle), "stdout"); f != nil {
			os.Stdout = f
		}
	}
	if stderrHandle != 0 {
		if f := os.NewFile(uintptr(stderrHandle), "stderr"); f != nil {
			os.Stderr = f
		}
	}
}
