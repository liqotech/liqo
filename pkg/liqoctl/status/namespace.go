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

package status

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

const nsCheckerName = "Namespace existence check"

// namespaceChecker implements the Check interface.
// checks if the namespace passed as an argument to liqoctl status command
// exists. If it does not exist the liqoctl status returns.
type namespaceChecker struct {
	options       *Options
	succeeded     bool
	failureReason error
}

func newNamespaceChecker(options *Options) *namespaceChecker {
	return &namespaceChecker{
		options: options,
	}
}

func (nc *namespaceChecker) Collect(ctx context.Context) error {
	// Check if the namespace exists.
	if _, err := nc.options.KubeClient.CoreV1().Namespaces().Get(ctx, nc.options.LiqoNamespace, v1.GetOptions{}); err != nil {
		nc.succeeded = false
		nc.failureReason = err
		return nil
	}

	nc.succeeded = true

	return nil
}

// GetTitle returns the title of the checker.
func (nc *namespaceChecker) GetTitle() string {
	return nsCheckerName
}

func (nc *namespaceChecker) Format() (string, error) {
	var text string
	if nc.succeeded {
		text += nc.options.Printer.Success.Sprint(pterm.Sprintf("%s liqo control plane namespace %s exists",
			nc.options.Printer.Success.Prefix.Style.Sprint(output.CheckMark),
			nc.options.Printer.Success.Prefix.Style.Sprint(nc.options.LiqoNamespace)))
		return text, nil
	}
	text += pterm.Sprintfln("%s liqo control plane namespace %s is not OK",
		nc.options.Printer.Error.Prefix.Style.Sprintf(output.Cross),
		nc.options.Printer.Error.Prefix.Style.Sprintf(nc.options.LiqoNamespace))
	text += nc.options.Printer.Paragraph.Sprintf("Reason: %s", nc.failureReason)
	text = nc.options.Printer.Error.Sprint(text)
	return text, fmt.Errorf("%s", text)
}

func (nc *namespaceChecker) HasSucceeded() bool {
	return nc.succeeded
}
