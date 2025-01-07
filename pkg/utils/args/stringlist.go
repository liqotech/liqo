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
	"strings"
)

// StringList implements the flag.Value interface and allows to parse stringified lists
// in the form: "val1,val2".
type StringList struct {
	StringList []string
}

// String returns the stringified list.
func (sl StringList) String() string {
	if sl.StringList == nil {
		return ""
	}
	return strings.Join(sl.StringList, ",")
}

// Set parses the provided string into the []string list.
func (sl *StringList) Set(str string) error {
	if sl.StringList == nil {
		sl.StringList = []string{}
	}
	if str == "" {
		return nil
	}
	chunks := strings.Split(str, ",")
	sl.StringList = append(sl.StringList, chunks...)
	return nil
}

// Type returns the stringList type.
func (sl StringList) Type() string {
	return "stringList"
}
