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

package util

import (
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var m1 = map[string]interface{}{
	"a": true,
	"b": map[string]interface{}{
		"c": "c1",
		"o": 20,
		"d": map[string]interface{}{
			"e1": map[string]interface{}{
				"f": "f1",
			},
		},
		"g": map[string]interface{}{
			"h": map[string]interface{}{
				"i": "i1",
			},
		},
		"slice": []interface{}{},
	},
}

var m2 = map[string]interface{}{
	"a": false,
	"b": map[string]interface{}{
		"z": "z1",
		"o": 30,
		"y": map[string]interface{}{
			"h": map[string]interface{}{
				"i": "i2",
			},
		},
		"d": map[string]interface{}{
			"e1": map[string]interface{}{
				"f":  "f2",
				"h2": "e2",
			},
		},
		"slice": []interface{}{"str1", "str2"},
	},
}

var expectedResultMap = map[string]interface{}{
	"a": false,
	"b": map[string]interface{}{
		"c": "c1",
		"z": "z1",
		"o": 30,
		"y": map[string]interface{}{
			"h": map[string]interface{}{
				"i": "i2",
			},
		},
		"d": map[string]interface{}{
			"e1": map[string]interface{}{
				"f":  "f2",
				"h2": "e2",
			},
		},
		"g": map[string]interface{}{
			"h": map[string]interface{}{
				"i": "i1",
			},
		},
		"slice": []interface{}{"str1", "str2"},
	},
}

func TestMergeMaps(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Map Fusion")
}

var _ = Describe("Test Map Fusion", func() {

	It("Returns a map with all expected keys and values", func() {
		finalMap, err := MergeMaps(m1, m2)
		Expect(err).ToNot(HaveOccurred())
		Expect(reflect.DeepEqual(finalMap, expectedResultMap)).To(BeTrue())
	})
})
