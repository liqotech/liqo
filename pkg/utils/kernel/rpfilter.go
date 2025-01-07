// Copyright 2019-2025 The Liqo Authors
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
	"os"
)

// DisableRtFilter disables the Reverse Path Filtering on the host.
func DisableRtFilter() error {
	rpFilterPaths := []string{
		"/proc/sys/net/ipv4/conf/all/rp_filter",
		"/proc/sys/net/ipv4/conf/default/rp_filter",
	}
	for _, rpFilterPath := range rpFilterPaths {
		if err := os.WriteFile(rpFilterPath, []byte("0"), os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}
