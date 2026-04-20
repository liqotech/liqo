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
	"fmt"
	"os"
)

const multipathpolicyFile = "/proc/sys/net/ipv4/fib_multipath_hash_policy"

// EnableMultipathHashPolicy enables 5-tuple hashing for multipath routing by writing 1 to /proc/sys/net/ipv4/fib_multipath_hash_policy.
func EnableMultipathHashPolicy() error {
	file, err := os.OpenFile(multipathpolicyFile, os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open multipath policy file %s: %w", multipathpolicyFile, err)
	}
	defer file.Close()

	if _, err := file.WriteString("1\n"); err != nil {
		return fmt.Errorf("failed to write to multipath policy file %s: %w", multipathpolicyFile, err)
	}
	return nil
}
