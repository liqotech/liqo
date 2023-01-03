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

package status

import (
	"context"
	"errors"

	"github.com/pterm/pterm"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
)

// Options encapsulates the arguments of the status command.
type Options struct {
	Verbose  bool
	Checkers []Checker
	*factory.Factory
}

// Run implements the logic of the status command.
func (o *Options) Run(ctx context.Context) error {
	for i, checker := range o.Checkers {
		checker.Collect(ctx)
		text := checker.Format()

		if !checker.Silent() || !checker.HasSucceeded() {
			o.Printer.BoxSetTitle(checker.GetTitle())
			o.Printer.BoxPrintln(text)
		}

		if !checker.HasSucceeded() {
			return errors.New("some checks failed")
		}
		// Insert a new line between each checker.
		if i != len(o.Checkers)-1 && !checker.Silent() {
			pterm.Println()
		}
	}
	return nil
}
