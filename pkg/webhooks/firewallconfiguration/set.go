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

package firewallconfiguration

import (
	"fmt"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	firewallutils "github.com/liqotech/liqo/pkg/firewall/utils"
)

func checkSetsInTable(sets []firewallapi.Set) error {
	err := checkUniqueSetNames(sets)
	if err != nil {
		return err
	}

	for i := range sets {
		if err := checkSetElements(&sets[i]); err != nil {
			return err
		}
	}

	return nil
}

func checkUniqueSetNames(sets []firewallapi.Set) error {
	names := map[string]interface{}{}
	for i := range sets {
		name := sets[i].Name
		if name == "" {
			return fmt.Errorf("set name is empty")
		}

		if _, ok := names[name]; ok {
			return fmt.Errorf("set name %v is duplicated", name)
		}

		names[name] = nil
	}
	return nil
}

func checkSetElements(set *firewallapi.Set) error {
	shouldHaveData := set.DataType != nil

	for i := range set.Elements {
		element := set.Elements[i]

		if _, err := firewallutils.ConvertSetData(&element.Key, &set.KeyType); err != nil {
			return err
		}

		if shouldHaveData {
			if element.Data == nil {
				return fmt.Errorf("set element with key %s has nil data", element.Key)
			}

			if _, err := firewallutils.ConvertSetData(element.Data, set.DataType); err != nil {
				return err
			}
		} else {
			if element.Data != nil {
				return fmt.Errorf("set element with key %s should not have data", element.Key)
			}
		}
	}

	return nil
}
