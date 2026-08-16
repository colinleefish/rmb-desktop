//go:build darwin

package launchatlogin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework ServiceManagement
#include <stdio.h>
#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>

static char rmb_sm_err[256] = {0};

// rmb_sm_supported reports whether SMAppService is available at runtime
// (macOS 13 Ventura introduced it).
static int rmb_sm_supported(void) {
	if (@available(macOS 13.0, *)) {
		return 1;
	}
	return 0;
}

// rmb_sm_register registers the calling process's bundle as a login item
// via SMAppService.mainApp. Returns 1 on success, 0 on failure (rmb_sm_err).
static int rmb_sm_register(void) {
	if (@available(macOS 13.0, *)) {
		NSError *err = nil;
		if ([[SMAppService mainAppService] registerAndReturnError:&err]) {
			rmb_sm_err[0] = 0;
			return 1;
		}
		if (err != nil) {
			snprintf(rmb_sm_err, sizeof(rmb_sm_err), "%s",
				[[err localizedDescription] UTF8String]);
		} else {
			snprintf(rmb_sm_err, sizeof(rmb_sm_err), "register failed");
		}
		return 0;
	}
	snprintf(rmb_sm_err, sizeof(rmb_sm_err), "SMAppService requires macOS 13+");
	return 0;
}

// rmb_sm_unregister removes the login-item registration.
static int rmb_sm_unregister(void) {
	if (@available(macOS 13.0, *)) {
		NSError *err = nil;
		if ([[SMAppService mainAppService] unregisterAndReturnError:&err]) {
			rmb_sm_err[0] = 0;
			return 1;
		}
		if (err != nil) {
			snprintf(rmb_sm_err, sizeof(rmb_sm_err), "%s",
				[[err localizedDescription] UTF8String]);
		} else {
			snprintf(rmb_sm_err, sizeof(rmb_sm_err), "unregister failed");
		}
		return 0;
	}
	snprintf(rmb_sm_err, sizeof(rmb_sm_err), "SMAppService requires macOS 13+");
	return 0;
}

// rmb_sm_status maps SMAppService.status for mainApp:
//   2 enabled, 1 requires approval, 0 not registered,
//  -1 not found (caller is not inside an app bundle),
//  -2 SMAppService unavailable (macOS < 13).
static int rmb_sm_status(void) {
	if (@available(macOS 13.0, *)) {
		switch ([[SMAppService mainAppService] status]) {
			case SMAppServiceStatusEnabled:
				return 2;
			case SMAppServiceStatusRequiresApproval:
				return 1;
			case SMAppServiceStatusNotFound:
				return -1;
			default:
				return 0;
		}
	}
	return -2;
}
// rmb_sm_last_error returns the buffer pointer so cgo never references the
// static symbol directly (that would produce an undefined external).
static const char* rmb_sm_last_error(void) {
	return rmb_sm_err;
}
*/
import "C"

import (
	"fmt"
	"os"
)

// SMAppService status codes (see rmb_sm_status above).
const (
	smStatusUnsupported      = -2
	smStatusNotFound         = -1
	smStatusNotRegistered    = 0
	smStatusRequiresApproval = 1
	smStatusEnabled          = 2
)

func smSupported() bool  { return C.rmb_sm_supported() == 1 }
func smStatus() int      { return int(C.rmb_sm_status()) }
func smLastError() error { return fmt.Errorf("%s", C.GoString(C.rmb_sm_last_error())) }

func smRegister() error {
	if C.rmb_sm_register() != 1 {
		return smLastError()
	}
	return nil
}

func smUnregister() error {
	if C.rmb_sm_unregister() != 1 {
		return smLastError()
	}
	return nil
}

// SetFromBundle applies the login-item state from the app bundle process via
// SMAppService.mainApp (macOS 13+). This is the preferred registration: the
// item shows up under "Open at Login > RMB Desktop" in System Settings and
// does not trigger the "software from <developer> can run in the background"
// notification that legacy LaunchAgents do.
//
// Fallbacks (both transparent):
//   - macOS < 13: legacy LaunchAgent plist.
//   - caller not inside an .app bundle (dev runs of bin/rmb-app): the modern
//     API reports NotFound; we fall back to the legacy plist so `make app-dev`
//     style workflows keep working.
func SetFromBundle(enabled bool) error {
	if !smSupported() {
		return set(enabled)
	}
	var err error
	if enabled {
		err = smRegister()
	} else {
		err = smUnregister()
	}
	if err != nil && smStatus() == smStatusNotFound {
		// Not running from an app bundle — no mainApp to register.
		return set(enabled)
	}
	return err
}

// BundleStatus exposes the SMAppService.mainApp status code. It is primarily
// for tests and diagnostics.
func BundleStatus() int { return smStatus() }

// MigrateFromLegacy removes the pre-SMAppService LaunchAgent (label
// me.remember.rmb.login) if its plist is present: boot it out, disable the
// label, delete the file. Returns hadLegacy=true when artifacts were found.
// The caller should afterwards apply the desired state via SetFromBundle —
// config.yaml remains the single source of truth.
func MigrateFromLegacy() (bool, error) {
	plistPath, err := plistPath()
	if err != nil {
		return false, err
	}
	if !fileExists(plistPath) {
		return false, nil
	}
	uid, err := guiUID()
	if err != nil {
		return false, err
	}
	domain := fmt.Sprintf("gui/%s", uid)
	bootout(domain, plistPath)
	_ = runLaunchctl("disable", fmt.Sprintf("%s/%s", domain, label))
	if err := os.Remove(plistPath); err != nil {
		return true, fmt.Errorf("remove legacy plist: %w", err)
	}
	return true, nil
}
