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

package status

import (
	"context"

	"github.com/pterm/pterm"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	liqoctlutil "github.com/liqotech/liqo/pkg/liqoctl/util"
)

// Options encapsulates the arguments of the status command.
type Options struct {
	Verbose  bool
	Checkers []Checker
	*factory.Factory
	NetworkingEnabled bool
}

// Run implements the logic of the status command.
func (o *Options) Run(ctx context.Context) error {
	if err := o.SetNetworkingEnabled(ctx); err != nil {
		return err
	}

	hasErrors := false
	for i, checker := range o.Checkers {
		checker.Collect(ctx)
		text := checker.Format()

		if !checker.Silent() || !checker.HasSucceeded() {
			o.Printer.BoxSetTitle(checker.GetTitle())
			o.Printer.BoxPrintln(text)
		}

		if !checker.HasSucceeded() {
			hasErrors = true
		}
		// Insert a new line between each checker.
		if i != len(o.Checkers)-1 && !checker.Silent() {
			pterm.Println()
		}
	}
	if hasErrors {
		o.Printer.Error.Println("some checks failed")
	}

	return nil
}

// SetNetworkingEnabled sets the internal network enabled flag.
func (o *Options) SetNetworkingEnabled(ctx context.Context) error {
	var ctrlargs []string
	ctrlargs, err := liqoctlutil.RetrieveLiqoControllerManagerDeploymentArgs(ctx, o.CRClient, o.LiqoNamespace)
	if err != nil {
		return err
	}
	value, err := liqoctlutil.ExtractValuesFromArgumentList("--networking-enabled", ctrlargs)
	if err != nil || value == "true" {
		o.NetworkingEnabled = true
	} else {
		o.NetworkingEnabled = false
	}
	return nil
}
