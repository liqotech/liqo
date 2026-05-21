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

package route

import (
	"crypto/sha256"
	"fmt"
)

// GetTableID returns the iproute2 table ID for the given table name.
// The ID is derived deterministically from the name via SHA256 and is guaranteed
// to be in [256, 65535], avoiding OS-reserved tables (0-255) and staying within
// the range accepted by all iproute2 versions.
// No entry is written to /etc/iproute2/rt_tables: the kernel does not require it
// and writing large IDs there causes iproute2 "database corrupted" errors on some
// distributions.
func GetTableID(tableName string) (uint32, error) {
	if tableName == "" {
		return 0, fmt.Errorf("table name is empty")
	}
	return generateTableID(tableName), nil
}

func generateTableID(name string) uint32 {
	hash := sha256.Sum256([]byte(name))
	id := hash[0:4]
	// the first bit of the most significant byte must be 0. https://serverfault.com/questions/315705/how-many-custom-route-tables-can-i-have-on-linux
	id[3] >>= 1
	// IDs in the range 0 <= ID <= 255 are used by the operating system
	// make sure we won't use this range
	id[1] |= 1
	return uint32(hash[3])<<24 | uint32(hash[2])<<16 | uint32(hash[1])<<8 | uint32(hash[0])
}
