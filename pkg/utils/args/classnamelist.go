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

// ClassName contains the name of a class and whether it is the default class.
type ClassName struct {
	Name      string
	IsDefault bool
}

// String returns the stringified class name.
func (cn ClassName) String() string {
	if cn.IsDefault {
		return fmt.Sprintf("%s;default", cn.Name)
	}
	return cn.Name
}

// ClassNameList implements the flag.Value interface and allows to parse stringified list of class names
// in the form: "class1;default,class2,class3".
type ClassNameList struct {
	Classes []ClassName
}

// String returns the stringified list.
func (cnl ClassNameList) String() string {
	if cnl.Classes == nil {
		return ""
	}
	var classes []string
	for _, c := range cnl.Classes {
		classes = append(classes, c.String())
	}
	return strings.Join(classes, ",")
}

// GetDefault returns the default class name, or the first class name if no default class is specified.
func (cnl *ClassNameList) GetDefault() string {
	if cnl.Classes == nil || len(cnl.Classes) == 0 {
		return ""
	}
	for _, c := range cnl.Classes {
		if c.IsDefault {
			return c.Name
		}
	}
	return cnl.Classes[0].Name
}

// Set parses the provided string into the []ClassName list.
func (cnl *ClassNameList) Set(str string) error {
	if cnl.Classes == nil {
		cnl.Classes = []ClassName{}
	}
	if str == "" {
		return nil
	}
	chunks := strings.Split(str, ",")
	for _, c := range chunks {
		if c == "" {
			continue
		}
		if strings.Contains(c, ";") {
			// The class is the default class.
			chunks := strings.Split(c, ";")
			cnl.Classes = append(cnl.Classes, ClassName{
				Name:      chunks[0],
				IsDefault: true,
			})
		} else {
			// The class is not the default class.
			cnl.Classes = append(cnl.Classes, ClassName{
				Name:      c,
				IsDefault: false,
			})
		}
	}
	return nil
}

// Type returns the classNameList type.
func (cnl ClassNameList) Type() string {
	return "classNameList"
}
