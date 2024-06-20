// Copyright 2019-2024 The Liqo Authors
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

package utils

import (
	"bytes"
	"reflect"
	"strconv"
	"text/template"
)

// RenderTemplate renders a template.
func RenderTemplate(obj, data interface{}, forceString bool) (interface{}, error) {
	// if the object is a string, render the template
	if reflect.TypeOf(obj).Kind() == reflect.String {
		tmpl, err := template.New("").Parse(obj.(string))
		if err != nil {
			return obj, err
		}

		res := bytes.NewBufferString("")
		if err := tmpl.Execute(res, data); err != nil {
			return obj, err
		}

		if !forceString {
			ret, err := strconv.Atoi(res.String())
			if err == nil {
				return ret, nil
			}
		}

		return res.String(), nil
	}

	// if the object is a map, render the template for each value
	if reflect.TypeOf(obj).Kind() == reflect.Map {
		for k, v := range obj.(map[string]interface{}) {
			res, err := RenderTemplate(v, data, forceString || isLabelsOrAnnotations(obj))
			if err != nil {
				return obj, err
			}

			obj.(map[string]interface{})[k] = res
		}

		return obj, nil
	}

	// if the object is a slice, render the template for each element
	if reflect.TypeOf(obj).Kind() == reflect.Slice {
		for i, v := range obj.([]interface{}) {
			res, err := RenderTemplate(v, data, forceString || isLabelsOrAnnotations(obj))
			if err != nil {
				return obj, err
			}

			obj.([]interface{})[i] = res
		}

		return obj, nil
	}

	return obj, nil
}

func isLabelsOrAnnotations(obj interface{}) bool {
	if reflect.TypeOf(obj).Kind() == reflect.Map {
		for k := range obj.(map[string]interface{}) {
			if k == "labels" || k == "annotations" {
				return true
			}
		}
	}

	return false
}
