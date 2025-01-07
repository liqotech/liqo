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

package utils

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

var variableRegex = regexp.MustCompile(`{{\s*(.\S+)\s*}}`)

type renderOptions struct {
	skipIfEmpty bool
}

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
			useKey, useValue, options := preProcessOptional(k, v, obj)

			res, err := RenderTemplate(useValue, data, forceString || isLabelsOrAnnotations(obj))
			if err != nil {
				return obj, err
			}

			if !(reflect.ValueOf(res).IsZero() && options.skipIfEmpty) {
				obj.(map[string]interface{})[useKey] = res
			}
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

	if forceString {
		return fmt.Sprintf("%v", obj), nil
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

// getVariableFromValue given a field value returns the first matched gotmpl variable.
func getVariableFromValue(value string) (string, bool) {
	// Look for variables in the value
	matches := variableRegex.FindStringSubmatch(value)

	// If a variable is found, than get only the first one
	if len(matches) > 1 {
		return matches[1], true
	}

	return "", false
}

// preProcessOptional preprocesses the template so that a field is rendered only if it has been provided.
func preProcessOptional(key string, value, obj interface{}) (newKey string, newValue interface{}, options renderOptions) {
	newKey = key
	newValue = value
	if strings.HasPrefix(key, "?") && reflect.TypeOf(key).Kind() == reflect.String {
		if variable, match := getVariableFromValue(value.(string)); match {
			newKey = key[1:]
			newValue = fmt.Sprintf("{{if %s}}%s{{end}}", variable, value)
			options.skipIfEmpty = true
			// Delete the field with the condition option
			delete(obj.(map[string]interface{}), key)
		}
	}

	return
}
