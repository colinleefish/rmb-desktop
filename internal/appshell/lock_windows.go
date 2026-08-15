//go:build windows

package appshell

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// Windows WaitForSingleObject event codes.
const (
	waitObject0 = 0 // WAIT_OBJECT_0
	waitTimeout = 258 // WAIT_TIMEOUT
)

// AcquireInstanceLock ports instance.rs (Windows branch): a global named
// mutex. The handle is intentionally leaked for the process lifetime, as in
// the Rust original.
func AcquireInstanceLock() (func(), error) {
	name, err := windows.UTF16PtrFromString(`Global\me.remember.rmb.app`)
	if err != nil {
		return nil, fmt.Errorf("CreateMutex failed: %w", err)
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		return nil, fmt.Errorf("CreateMutex failed: %w", err)
	}
	// ERROR_ALREADY_EXISTS still returns a valid handle; probe ownership.
	event, werr := windows.WaitForSingleObject(handle, 0)
	switch {
	case event == waitTimeout:
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("RMB is already running")
	case event != waitObject0:
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("WaitForSingleObject failed: %d", event)
	case werr != nil:
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("WaitForSingleObject failed: %w", werr)
	}

	release := func() {
		windows.ReleaseMutex(handle)
		windows.CloseHandle(handle)
	}
	return release, nil
}
