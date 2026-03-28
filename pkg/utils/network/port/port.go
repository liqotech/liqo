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

package port

import (
	"fmt"
	"strconv"
	"strings"
)

// ParsePortRange parses the port range and returns the start and end of the range.
func ParsePortRange(value string) (start, end uint16, err error) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid port range %s", value)
	}

	s, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start port: %w", err)
	}

	e, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end port: %w", err)
	}

	return uint16(s), uint16(e), nil
}
