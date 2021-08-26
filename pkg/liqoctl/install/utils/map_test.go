package installutils

import (
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo"
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

func TestFusionMaps(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Map Fusion")
}

var _ = Describe("Test Map Fusion", func() {

	It("Returns a map with all expected keys and values", func() {
		finalMap, err := FusionMap(m1, m2)
		Expect(err).ToNot(HaveOccurred())
		Expect(reflect.DeepEqual(finalMap, expectedResultMap)).To(BeTrue())
	})
})
