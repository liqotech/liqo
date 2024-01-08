// Copyright 2019-2023 The Liqo Authors
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

package identity

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/printers"

	"github.com/liqotech/liqo/pkg/liqoctl/rest"
)

// Create creates a VirtualNode.
func (o *Options) Create(_ context.Context, _ *rest.CreateOptions) *cobra.Command {
	panic("not implemented")
}

// output implements the logic to output the generated Configuration resource.
func (o *Options) output(conf *corev1.Secret) error {
	var outputFormat string
	switch {
	case o.generateOptions != nil:
		outputFormat = o.generateOptions.OutputFormat
	default:
		return fmt.Errorf("unable to determine output format")
	}
	var printer printers.ResourcePrinter
	switch outputFormat {
	case yamlLabel:
		printer = &printers.YAMLPrinter{}
	case jsonLabel:
		printer = &printers.JSONPrinter{}
	default:
		return fmt.Errorf("unsupported output format %q", outputFormat)
	}

	return printer.PrintObj(conf, os.Stdout)
}
