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

package tenant

import (
	"fmt"
	"os"

	"k8s.io/cli-runtime/pkg/printers"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
)

// output implements the logic to output the generated tenant resource.
func (o *Options) output(tenant *authv1beta1.Tenant) error {
	var outputFormat string
	switch {
	case o.generateOptions != nil:
		outputFormat = o.generateOptions.OutputFormat
	default:
		return fmt.Errorf("unable to determine output format")
	}
	var printer printers.ResourcePrinter
	switch outputFormat {
	case "yaml":
		printer = &printers.YAMLPrinter{}
	case "json":
		printer = &printers.JSONPrinter{}
	default:
		return fmt.Errorf("unsupported output format %q", outputFormat)
	}

	return printer.PrintObj(tenant, os.Stdout)
}
