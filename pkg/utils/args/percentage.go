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

package args

import (
	"fmt"
	"strconv"
)

// Percentage implements the flag.Value interface and allows to parse stringified percentages.
type Percentage struct {
	Val uint64
}

// String returns the stringified percentage.
func (p Percentage) String() string {
	return fmt.Sprintf("%v", p.Val)
}

// Set parses the provided string into the percentage.
func (p *Percentage) Set(str string) error {
	if str == "" {
		return nil
	}
	val, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return err
	}
	p.Val = val

	if p.Val > 100 {
		return fmt.Errorf("invalid percentage value: %v. It has to be in range [0 - 100]", str)
	}

	return nil
}

// Type returns the percentage type.
func (p Percentage) Type() string {
	return "percentage"
}
