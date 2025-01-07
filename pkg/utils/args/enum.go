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
	"strings"
)

// StringEnum is a type used to validate a string parameter is included in a set of allowed values.
type StringEnum struct {
	Allowed []string
	Value   string
}

// NewEnum give a list of allowed flag parameters, where the second argument is the default.
func NewEnum(allowed []string, d string) *StringEnum {
	return &StringEnum{
		Allowed: allowed,
		Value:   d,
	}
}

// NewEnumWithVoidDefault give a list of allowed flag parameters, where the default is a void string.
func NewEnumWithVoidDefault(allowed []string) *StringEnum {
	return &StringEnum{
		Allowed: allowed,
		Value:   "",
	}
}

// String returns the stringified value.
func (a StringEnum) String() string {
	return a.Value
}

// Set parses the provided string checking its validity and setting it inside the Value field.
func (a *StringEnum) Set(p string) error {
	isIncluded := func(opts []string, val string) bool {
		for _, opt := range opts {
			if val == opt {
				return true
			}
		}
		return false
	}
	if !isIncluded(a.Allowed, p) {
		return fmt.Errorf("%s is not included in %s", p, strings.Join(a.Allowed, ","))
	}
	a.Value = p
	return nil
}

// Type returns the enum type.
func (a *StringEnum) Type() string {
	return "string"
}
