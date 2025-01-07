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

// StringMap implements the flag.Value interface and allows to parse stringified maps
// in the form: "key1=val1,key2=val2". Values can contain additional '=' characters,
// for example: "argocd.argoproj.io/sync-options=Prune=false". In this case, only the
// first '=' is used as the key-value separator.
type StringMap struct {
	StringMap map[string]string
}

// String returns the stringified map.
func (sm StringMap) String() string {
	if sm.StringMap == nil {
		return ""
	}

	strs := make([]string, len(sm.StringMap))
	i := 0
	for k, v := range sm.StringMap {
		strs[i] = fmt.Sprintf("%s=%s", k, v)
		i++
	}
	return strings.Join(strs, ",")
}

// Set parses the provided string into the map[string]string map.
// The input string is expected to be in the format "key1=val1,key2=val2,...".
// Multiple '=' characters in the value part are preserved, only the first '='
// is used as the key-value separator.
func (sm *StringMap) Set(str string) error {
	if sm.StringMap == nil {
		sm.StringMap = map[string]string{}
	}
	if str == "" {
		return nil
	}
	// Split the input string into chunks using comma as separator
	chunks := strings.Split(str, ",")
	for i := range chunks {
		chunk := chunks[i]
		// Find the position of the first '=' character which separates key from value
		idx := strings.Index(chunk, "=")
		if idx == -1 {
			return fmt.Errorf("invalid value %v: missing '=' separator", chunk)
		}
		// Extract key and value using the position of the first '='
		// Any subsequent '=' characters will be part of the value
		key := chunk[:idx]
		value := chunk[idx+1:]
		sm.StringMap[key] = value
	}
	return nil
}

// Type returns the stringMap type.
func (sm StringMap) Type() string {
	return "stringMap"
}
