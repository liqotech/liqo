package installutils

import (
	"fmt"
	"reflect"
)

// FusionMap fusions two maps recursively writing the result in expectedResultMap result passed as argument. In case of duplicated keys,
// the values extracted from patchMap are considered valid.
func FusionMap(baseMap, patchMap map[string]interface{}) (map[string]interface{}, error) {
	var err error
	resultMap := make(map[string]interface{})
	for _, key := range extractKeys(baseMap, patchMap) {
		v, ok := baseMap[key]
		v2, ok2 := patchMap[key]

		if ok && !ok2 {
			resultMap[key] = v
			continue
		} else if !ok && ok2 {
			resultMap[key] = v2
			continue
		}

		switch {
		case reflect.TypeOf(v) != reflect.TypeOf(v2):
			return nil, fmt.Errorf("the two maps have different types for the same key")
		case reflect.TypeOf(v).String() == "string", reflect.TypeOf(v).String() == "bool", reflect.TypeOf(v).String() == "int":
			resultMap[key] = v2
		case reflect.TypeOf(v).Kind() == reflect.Slice:
			resultMap[key] = append(v.([]interface{}), v2.([]interface{})...)
		default:
			resultMap[key], err = FusionMap(baseMap[key].(map[string]interface{}), patchMap[key].(map[string]interface{}))
			if err != nil {
				return nil, err
			}
		}
	}

	return resultMap, nil
}

func extractKeys(baseMap, patchMap map[string]interface{}) []string {
	keys := make(map[string]interface{})
	for k := range baseMap {
		keys[k] = struct{}{}
	}

	for k := range patchMap {
		keys[k] = struct{}{}
	}

	keysArr := make([]string, len(keys))
	i := 0
	for k := range keys {
		keysArr[i] = k
		i++
	}
	return keysArr
}
