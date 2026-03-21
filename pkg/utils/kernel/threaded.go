// Copyright 2019-2026 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kernel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

const (
	// MaxMountAttempts is the default number of retries when remounting sysfs.
	MaxMountAttempts = 100
	// MountBackoffBase is the baseline duration between mount retry attempts.
	MountBackoffBase = 100 * time.Millisecond
)

// EnableWireguardThreadedMode ensures that threaded NAPI is enabled for the given WireGuard interface.
// It writes 1 to /sys/class/net/<ifaceName>/threaded (only if the feature is not already enabled).
func EnableWireguardThreadedMode(ifaceName string) (bool, error) {
	path := fmt.Sprintf("/sys/class/net/%s/threaded", ifaceName)

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return false, err
	}

	if len(data) > 0 && data[0] == '1' {
		return false, nil
	}

	if err := os.WriteFile(path, []byte("1\n"), 0o600); err != nil {
		return false, err
	}

	return true, nil
}

// RemountSysfsRW remounts /sys as read-write, retrying up to 100 times.
func RemountSysfsRW() error {
	for attempt := range MaxMountAttempts {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * MountBackoffBase)
		}
		err := unix.Mount("sysfs", "/sys", "sysfs", unix.MS_REMOUNT, "rw")
		if err == nil {
			klog.Infof("Remounted /sys as read-write after %d/%d retries", attempt+1, MaxMountAttempts)
			return nil
		}
		klog.Infof("Failed to remount /sys as read-write (attempt %d/%d): %v", attempt+1, MaxMountAttempts, err)
	}
	return fmt.Errorf("failed to remount /sys as read-write after %d attempts", MaxMountAttempts)
}

// RemountSysfsRO remounts /sys as read-only for security, retrying indefinitely until successful.
func RemountSysfsRO() {
	for {
		err := unix.Mount("sysfs", "/sys", "sysfs", unix.MS_REMOUNT|unix.MS_RDONLY, "")
		if err == nil {
			klog.Info("Successfully remounted /sys as read-only")
			break
		}

		klog.Infof("Failed to remount /sys as read-only, retrying in %v...: %v", MountBackoffBase, err)
		time.Sleep(MountBackoffBase)
	}
}

// IsThreadedNAPISupported checks if the kernel supports threaded NAPI by probing the sysfs entry for the given interface.
func IsThreadedNAPISupported(iface string) bool {
	path := fmt.Sprintf("/sys/class/net/%s/threaded", iface)
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err != nil {
		klog.Warningf("Unexpected error checking threaded NAPI support for %s: %v", iface, err)
		return false
	}
	return true
}
