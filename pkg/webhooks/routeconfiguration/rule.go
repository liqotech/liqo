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

package routeconfiguration

import (
	"fmt"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

func checkUniqueRules(rules []networkingv1beta1.Rule) error {
	uniqueKeys := make(map[string]interface{})
	for i := range rules {
		key := ""
		if rules[i].Src != nil {
			key += fmt.Sprintf("src:%s,", rules[i].Src.String())
		}
		if rules[i].Dst != nil {
			key += fmt.Sprintf("dst:%s,", rules[i].Dst.String())
		}
		if rules[i].Iif != nil {
			key += fmt.Sprintf("iif:%s,", *rules[i].Iif)
		}
		if rules[i].Oif != nil {
			key += fmt.Sprintf("oif:%s,", *rules[i].Oif)
		}
		if rules[i].FwMark != nil {
			key += fmt.Sprintf("mark:%d,", *rules[i].FwMark)
		}
		if _, ok := uniqueKeys[key]; ok {
			return fmt.Errorf("cannot insert replicated rules: %s", key)
		}
		uniqueKeys[key] = nil
	}
	return nil
}
