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

func checkUniqueRoutes(routes []networkingv1beta1.Route) error {
	uniqueKeys := make(map[string]interface{})
	for i := range routes {
		if _, ok := uniqueKeys[routes[i].Dst.String()]; ok {
			return fmt.Errorf("cannot insert replicated destinantion in same rule, %s already used", routes[i].Dst.String())
		}
		uniqueKeys[routes[i].Dst.String()] = nil
	}
	return nil
}
