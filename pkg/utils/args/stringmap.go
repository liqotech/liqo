package args

import (
	"fmt"
	"strings"
)

// StringMap implements the flag.Value interface and allows to parse stringified maps
// in the form: "key1=val1,key2=val2".
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
func (sm *StringMap) Set(str string) error {
	if sm.StringMap == nil {
		sm.StringMap = map[string]string{}
	}
	if str == "" {
		return nil
	}
	chunks := strings.Split(str, ",")
	for i := range chunks {
		chunk := chunks[i]
		strs := strings.Split(chunk, "=")
		if len(strs) != 2 {
			return fmt.Errorf("invalid value %v", chunk)
		}
		sm.StringMap[strs[0]] = strs[1]
	}
	return nil
}

// Type returns the stringMap type.
func (sm StringMap) Type() string {
	return "stringMap"
}
