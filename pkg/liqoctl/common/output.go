// Copyright 2019-2022 The Liqo Authors
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

package common

import "github.com/pterm/pterm"

var (
	spinnerCharset = []string{"⠈⠁", "⠈⠑", "⠈⠱", "⠈⡱", "⢀⡱", "⢄⡱", "⢄⡱", "⢆⡱",
		"⢎⡱", "⢎⡰", "⢎⡠", "⢎⡀", "⢎⠁", "⠎⠁", "⠊⠁"}

	// GenericPrinter used to print generic output.
	GenericPrinter = pterm.PrefixPrinter{
		Prefix: pterm.Prefix{},
		Scope: pterm.Scope{
			Text:  "cmd",
			Style: pterm.NewStyle(pterm.FgGray),
		},
		MessageStyle: pterm.NewStyle(pterm.FgDefault),
	}

	// SuccessPrinter used to print success output.
	SuccessPrinter = GenericPrinter.WithPrefix(pterm.Prefix{
		Text:  "[SUCCESS]",
		Style: pterm.NewStyle(pterm.FgGreen),
	})

	// WarningPrinter used to print warning output.
	WarningPrinter = GenericPrinter.WithPrefix(pterm.Prefix{
		Text:  "[WARNING]",
		Style: pterm.NewStyle(pterm.FgYellow),
	})

	// ErrorPrinter used to print error output.
	ErrorPrinter = GenericPrinter.WithPrefix(pterm.Prefix{
		Text:  "[ERROR]",
		Style: pterm.NewStyle(pterm.FgRed),
	})
)

// Printer manages all kinds of outputs.
type Printer struct {
	Info        *pterm.PrefixPrinter
	Success     *pterm.PrefixPrinter
	Warning     *pterm.PrefixPrinter
	Error       *pterm.PrefixPrinter
	Spinner     *pterm.SpinnerPrinter
	InfoMessage string
}
