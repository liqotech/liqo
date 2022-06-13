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

package output

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/pterm/pterm"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubectl/pkg/cmd/util"
)

func init() {
	// Disable styling if we are not in a standard terminal, as control sequences would not work.
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		pterm.DisableStyling()
	}
}

const (
	localClusterName  = "local"
	remoteClusterName = "remote"

	localClusterColor  = pterm.FgLightBlue
	remoteClusterColor = pterm.FgLightMagenta
)

var spinnerCharset = []string{"⠈⠁", "⠈⠑", "⠈⠱", "⠈⡱", "⢀⡱", "⢄⡱", "⢄⡱", "⢆⡱", "⢎⡱", "⢎⡰", "⢎⡠", "⢎⡀", "⢎⠁", "⠎⠁", "⠊⠁"}

// Printer manages all kinds of outputs.
type Printer struct {
	Info    *pterm.PrefixPrinter
	Success *pterm.PrefixPrinter
	Warning *pterm.PrefixPrinter
	Error   *pterm.PrefixPrinter

	spinner *pterm.SpinnerPrinter
	verbose bool
}

// StartSpinner starts a new spinner.
func (p *Printer) StartSpinner(text ...interface{}) *pterm.SpinnerPrinter {
	spinner, err := p.spinner.Start(text...)
	utilruntime.Must(err)
	return spinner
}

// Verbosef outputs verbose messages guarded by the corresponding flag.
func (p *Printer) Verbosef(format string, args ...interface{}) {
	if p.verbose {
		p.Info.Printfln(strings.TrimRight(format, "\n"), args...)
	}
}

// CheckErr prints a user friendly error and exits with a non-zero exit code.
// If a spinner is currently active, then it is leveraged to print the message,
// otherwise it outputs the message through the printer or, if nil, to STDERR.
func (p *Printer) CheckErr(err error) {
	switch {
	// Shortcircuit in case no error occurred.
	case err == nil:
		return

	// Print the error through the spinner, if specified.
	case p != nil && p.spinner.IsActive:
		util.BehaviorOnFatal(func(msg string, code int) {
			p.spinner.Fail(msg)
			os.Exit(code)
		})

	// Print the error through the printer, if initialized.
	case p != nil:
		util.BehaviorOnFatal(func(msg string, code int) {
			p.Error.Println(strings.TrimRight(msg, "\n"))
			os.Exit(code)
		})

	// Otherwise, restore the default behavior.
	default:
		util.DefaultBehaviorOnFatal()
	}

	util.CheckErr(err)
}

// PrettyErr returns a prettified error message, according to standard kubectl style.
func PrettyErr(err error) string {
	// Unwrap possible URL errors, to return the prettified message.
	urlErr := &url.Error{}
	if errors.As(err, &urlErr) {
		err = urlErr
	}

	if msg, ok := util.StandardErrorMessage(err); ok {
		return msg
	}

	return strings.Replace(err.Error(), context.DeadlineExceeded.Error(), "timed out waiting for the condition", 1)
}

// ExitOnErr aborts the execution in case of errors, without printing any error message.
func ExitOnErr(err error) {
	if err != nil {
		os.Exit(util.DefaultErrorExitCode)
	}
}

// NewLocalPrinter returns a new printer referring to the local cluster.
func NewLocalPrinter(scoped, verbose bool) *Printer {
	return newPrinter(localClusterName, localClusterColor, scoped, verbose)
}

// NewRemotePrinter returns a new printer referring to the remote cluster.
func NewRemotePrinter(scoped, verbose bool) *Printer {
	return newPrinter(remoteClusterName, remoteClusterColor, scoped, verbose)
}

func newPrinter(scope string, color pterm.Color, scoped, verbose bool) *Printer {
	generic := pterm.PrefixPrinter{MessageStyle: pterm.NewStyle(pterm.FgDefault)}

	if scoped {
		generic.WithScope(pterm.Scope{Text: scope, Style: pterm.NewStyle(pterm.FgGray)})
	}

	printer := &Printer{
		verbose: verbose,

		Info: generic.WithPrefix(pterm.Prefix{
			Text:  "INFO",
			Style: pterm.NewStyle(pterm.FgDarkGray),
		}),

		Success: generic.WithPrefix(pterm.Prefix{
			Text:  "INFO",
			Style: pterm.NewStyle(pterm.FgGreen),
		}),

		Warning: generic.WithPrefix(pterm.Prefix{
			Text:  "WARN",
			Style: pterm.NewStyle(pterm.FgYellow),
		}),

		Error: generic.WithPrefix(pterm.Prefix{
			Text:  "ERRO",
			Style: pterm.NewStyle(pterm.FgRed),
		}),
	}

	printer.spinner = &pterm.SpinnerPrinter{
		Sequence:            spinnerCharset,
		Style:               pterm.NewStyle(color),
		Delay:               time.Millisecond * 100,
		MessageStyle:        pterm.NewStyle(color),
		SuccessPrinter:      printer.Success,
		WarningPrinter:      printer.Warning,
		FailPrinter:         printer.Error,
		RemoveWhenDone:      false,
		ShowTimer:           true,
		TimerRoundingFactor: time.Second,
		TimerStyle:          &pterm.ThemeDefault.TimerStyle,
	}

	return printer
}

// NewFakePrinter returns a new printer to be used in tests.
func NewFakePrinter(writer io.Writer) *Printer {
	printer := newPrinter("fake", pterm.FgBlack, true, true)
	printer.Info.Writer = writer
	printer.Success.Writer = writer
	printer.Warning.Writer = writer
	printer.Error.Writer = writer
	return printer
}
