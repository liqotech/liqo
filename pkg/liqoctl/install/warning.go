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

package install

import (
	"fmt"

	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"

	"github.com/liqotech/liqo/pkg/liqoctl/utils"
)

type warner interface {
	name() string
	check(values map[string]interface{}) (bool, error)
	warn() []string
}

type serviceWarner struct {
	warnings []string
}

var _ warner = &serviceWarner{}

func (sw *serviceWarner) name() string {
	return "Service type"
}

func (sw *serviceWarner) check(values map[string]interface{}) (bool, error) {
	var value interface{}
	var svctype string
	var err error
	var ok bool

	components := []string{"gateway", "auth"}

	for _, component := range components {
		if value, err = utils.ExtractValuesFromNestedMaps(values, component, "service", "type"); err != nil {
			return false, err
		}

		if svctype, ok = value.(string); !ok {
			return false, fmt.Errorf("cannot cast %v to string", value)
		}

		if corev1.ServiceType(svctype) == corev1.ServiceTypeClusterIP {
			sw.warnings = append(sw.warnings, fmt.Sprintf(
				"Service type of %s is %s. It will not be reachable from outside the cluster",
				pterm.Bold.Sprintf("liqo-%s", component), pterm.Bold.Sprintf("%s", svctype)))
		}
	}
	return len(sw.warnings) == 0, nil
}

func (sw *serviceWarner) warn() []string {
	return sw.warnings
}

// ValuesWarning checks the values map and returns a list of warnings.
func ValuesWarning(values map[string]interface{}) ([]string, error) {
	warners := []warner{&serviceWarner{}}
	warnings := []string{}
	for i := range warners {
		ok, err := warners[i].check(values)
		if err != nil {
			return warnings, fmt.Errorf("cannot check %s: %w", warners[i].name(), err)
		}
		if !ok {
			warnings = append(warnings, warners[i].warn()...)
		}
	}
	return warnings, nil
}
