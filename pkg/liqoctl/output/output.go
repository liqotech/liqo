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

	levelMultiplier = 4

	// CheckMark is the unicode checkmark.
	CheckMark = "✔"
	// Cross is the unicode cross.
	Cross = "✖"

	boxWidth = 80
)

var (
	// StatusSectionStyle is the style of the status section.
	StatusSectionStyle = pterm.NewStyle(pterm.FgMagenta, pterm.Bold)
	// StatusSectionSuccessStyle is the style of the success status section.
	StatusSectionSuccessStyle = pterm.NewStyle(pterm.FgGreen, pterm.Bold)
	// StatusSectionFailureStyle is the style of the failure status section.
	StatusSectionFailureStyle = pterm.NewStyle(pterm.FgRed, pterm.Bold)
	// StatusSectionInfoStyle is the style of the info status section.
	StatusSectionInfoStyle = pterm.NewStyle(pterm.FgDefault, pterm.Bold)
	// StatusDataStyle is the style of the status data.
	StatusDataStyle = pterm.NewStyle(pterm.FgDefault, pterm.Bold)
	// StatusInfoStyle is the style of the status info.
	StatusInfoStyle = pterm.NewStyle(pterm.FgCyan, pterm.Bold)
	// StatusWarningStyle is the style of the status info.
	StatusWarningStyle = pterm.NewStyle(pterm.FgYellow, pterm.Bold)
	// BoxTitleStyle is the style of the box.
	BoxTitleStyle = pterm.NewStyle(pterm.FgMagenta, pterm.Bold)
)

var spinnerCharset = []string{"⠈⠁", "⠈⠑", "⠈⠱", "⠈⡱", "⢀⡱", "⢄⡱", "⢄⡱", "⢆⡱", "⢎⡱", "⢎⡰", "⢎⡠", "⢎⡀", "⢎⠁", "⠎⠁", "⠊⠁"}

var confirm = &pterm.InteractiveConfirmPrinter{
	DefaultValue: false,
	DefaultText:  "Are you sure you want to continue?",
	TextStyle:    pterm.NewStyle(pterm.FgYellow, pterm.Bold),
	ConfirmText:  "yes",
	ConfirmStyle: pterm.NewStyle(pterm.FgDefault),
	RejectText:   "no",
	RejectStyle:  pterm.NewStyle(pterm.FgDefault),
	SuffixStyle:  pterm.NewStyle(pterm.FgDefault, pterm.Bold),
}

// Printer manages all kinds of outputs.
type Printer struct {
	Info    *pterm.PrefixPrinter
	Success *pterm.PrefixPrinter
	Warning *pterm.PrefixPrinter
	Error   *pterm.PrefixPrinter

	box        *pterm.BoxPrinter
	spinner    *pterm.SpinnerPrinter
	BulletList *pterm.BulletListPrinter
	Section    *pterm.SectionPrinter
	Paragraph  *pterm.ParagraphPrinter
	Logger     *pterm.Logger
	Table      *pterm.TablePrinter
	verbose    bool
}

// SpinnerRunningWarning prints a warning message while a spinner is running.
// It returns a new spinner printer which must be used instead of the one passed in the arguments.
func (p *Printer) SpinnerRunningWarning(spinner *pterm.SpinnerPrinter, message ...interface{}) *pterm.SpinnerPrinter {
	spinner.Warning(message...)
	return p.StartSpinner(spinner.Text)
}

// SpinnerRunningSuccess prints a success message while a spinner is running.
// It returns a new spinner printer which must be used instead of the one passed in the arguments.
func (p *Printer) SpinnerRunningSuccess(spinner *pterm.SpinnerPrinter, message ...interface{}) *pterm.SpinnerPrinter {
	spinner.Success(message...)
	return p.StartSpinner(spinner.Text)
}

// AskConfirm asks the user to confirm an action.
func (p *Printer) AskConfirm(cmdName string, skip bool) error {
	if skip {
		return nil
	}
	pterm.NewStyle(pterm.FgYellow, pterm.Bold).Printfln("%s is a potentially destructive command.", cmdName)
	r, e := confirm.Show()
	if e != nil || !r {
		return errors.New("action aborted")
	}
	return nil
}

// BoxPrintln prints a message through the box printer.
func (p *Printer) BoxPrintln(text string) {
	// create a string long as the box width
	widthLine := strings.Repeat("-", boxWidth)
	// insert widthLine inside the text
	text = pterm.Sprintf("%s\n%s", widthLine, text)
	// print the box with widthLine inside to force the box width
	boxText := p.box.Sprintln(text)
	// remove the widthLine (first line) from boxText
	widthLine = strings.Split(boxText, "\n")[1]
	boxText = strings.ReplaceAll(boxText, widthLine+"\n", "")
	pterm.Print(boxText)
}

// BoxSetTitle sets the title of the box.
func (p *Printer) BoxSetTitle(title string) {
	p.box.Title = BoxTitleStyle.Sprint(title)
}

// BulletListSprintForBox prints the bullet list for the box.
func (p *Printer) BulletListSprintForBox() string {
	// Srender function never throws an error.
	text, err := p.BulletList.Srender()
	// Flush Items to avoid printing the same list twice.
	p.BulletList.Items = []pterm.BulletListItem{}
	p.CheckErr(err)
	text = strings.TrimRight(text, "\n")
	return text
}

// BulletListAddItem adds a new message to the BulletListPrinter.
func (p *Printer) bulletListAddItem(msg string, level int, bullet bool) {
	bulletListItem := pterm.BulletListItem{
		Text:  msg,
		Level: level * levelMultiplier,
	}
	if bullet {
		bulletListItem.Bullet = " " + pterm.DefaultBulletList.Bullet
	}
	p.BulletList.Items = append(p.BulletList.Items, bulletListItem)
}

// BulletListAddItemWithoutBullet adds a new message to the BulletListPrinter.
func (p *Printer) BulletListAddItemWithoutBullet(msg string, level int) {
	p.bulletListAddItem(msg, level, false)
}

// BulletListAddItemWithBullet adds a new message to the BulletListPrinter.
func (p *Printer) BulletListAddItemWithBullet(msg string, level int) {
	p.bulletListAddItem(msg, level, true)
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

// ExitWithMessage prints the error message and exits with a non-zero exit code.
func (p *Printer) ExitWithMessage(errmsg string) {
	p.Error.Println(errmsg)
	os.Exit(util.DefaultErrorExitCode)
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

// NewGlobalPrinter returns a new printer referring to the global scope.
func NewGlobalPrinter(scoped, verbose bool) *Printer {
	return newPrinter("global", pterm.FgDefault, scoped, verbose)
}

func newPrinter(scope string, color pterm.Color, scoped, verbose bool) *Printer {
	generic := &pterm.PrefixPrinter{
		MessageStyle: pterm.NewStyle(pterm.FgDefault),
		Writer:       os.Stderr,
	}

	if scoped {
		generic = generic.WithScope(pterm.Scope{Text: scope, Style: pterm.NewStyle(pterm.FgGray)})
	}

	printer := &Printer{
		verbose: verbose,
		Info: generic.WithPrefix(pterm.Prefix{
			Text:  "INFO",
			Style: pterm.NewStyle(pterm.FgDarkGray, pterm.Bold),
		}),

		Success: generic.WithPrefix(pterm.Prefix{
			Text:  "INFO",
			Style: pterm.NewStyle(pterm.FgGreen, pterm.Bold),
		}),

		Warning: generic.WithPrefix(pterm.Prefix{
			Text:  "WARN",
			Style: pterm.NewStyle(pterm.FgYellow, pterm.Bold),
		}),

		Error: generic.WithPrefix(pterm.Prefix{
			Text:  "ERRO",
			Style: pterm.NewStyle(pterm.FgRed, pterm.Bold),
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
		Writer:              os.Stderr,
	}

	printer.BulletList = &pterm.BulletListPrinter{}

	printer.Section = &pterm.SectionPrinter{
		Style: StatusSectionStyle,
		Level: 1,
	}

	printer.box = &pterm.DefaultBox
	printer.Paragraph = &pterm.ParagraphPrinter{
		MaxWidth: boxWidth - len(pterm.RemoveColorFromString(printer.Error.Prefix.Text)) - 3,
	}

	printer.Logger = pterm.DefaultLogger.WithTime(false)

	printer.Table = pterm.DefaultTable.WithHasHeader().WithBoxed()

	return printer
}

// NewFakePrinter returns a new printer to be used in tests.
func NewFakePrinter(writer io.Writer) *Printer {
	printer := newPrinter("fake", pterm.FgBlack, true, true)
	printer.Info.Writer = writer
	printer.Success.Writer = writer
	printer.Warning.Writer = writer
	printer.Error.Writer = writer
	printer.BulletList.Writer = writer
	printer.Section.Writer = writer
	printer.box.Writer = writer
	return printer
}
