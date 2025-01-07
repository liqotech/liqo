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

package remotemetrics

import (
	"fmt"
	"strings"
)

type namespaceMapper struct {
	namespaces []MappedNamespace
}

// NewNamespaceMapper returns a new namespace mapper.
func NewNamespaceMapper(namespaces ...MappedNamespace) Mapper {
	mappedNamespaces := []MappedNamespace{}
	for i := range namespaces {
		if namespaces[i].Namespace != namespaces[i].OriginalName {
			mappedNamespaces = append(mappedNamespaces, namespaces[i])
		}
	}

	return &namespaceMapper{
		namespaces: mappedNamespaces,
	}
}

// Map returns the mapped metric line translating the namespace name with the original one.
func (nm *namespaceMapper) Map(line string) string {
	for _, n := range nm.namespaces {
		if strings.Contains(line, fmt.Sprintf("namespace=%q", n.Namespace)) {
			return strings.Replace(line,
				fmt.Sprintf("namespace=%q", n.Namespace), fmt.Sprintf("namespace=%q", n.OriginalName), 1)
		}
	}
	return line
}
