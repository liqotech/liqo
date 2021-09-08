// Copyright 2019-2021 The Liqo Authors
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
			return nil, fmt.Errorf("the two maps have different types for the same key %v %v", reflect.TypeOf(v), reflect.TypeOf(v2))
		case reflect.TypeOf(v).String() == "string", reflect.TypeOf(v).String() == "bool",
			reflect.TypeOf(v).String() == "int", reflect.TypeOf(v).String() == "float64":
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
