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
//

package utils

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/cmd/util"
)

var _commandName string

func init() {
	_commandName = getCommandName()
}

// GetCommandName gets the command name to be used in the help message.
func GetCommandName() string {
	return _commandName
}

// DescWithTemplate returns a string that has the liqoctl name templated out with the
// current executable name. DescWithTemplate templates on the '{{ .Executable }}' variable.
func DescWithTemplate(str, executable string) string {
	tmpl := template.Must(template.New("liqoctl").Parse(str))
	var buf bytes.Buffer
	util.CheckErr(tmpl.Execute(&buf, struct{ Executable string }{executable}))
	return buf.String()
}

// PatchCommandWithTemplate patches the command description with the current executable name.
func PatchCommandWithTemplate(cmd *cobra.Command) *cobra.Command {
	cmd.Short = DescWithTemplate(cmd.Short, GetCommandName())
	cmd.Long = DescWithTemplate(cmd.Long, GetCommandName())
	cmd.Example = DescWithTemplate(cmd.Example, GetCommandName())
	return cmd
}

// getCommandName gets the command name to be used in the help message.
func getCommandName() string {
	liqoctl := os.Args[0]

	// Account for the case it is used as a kubectl plugin.
	if strings.HasPrefix(filepath.Base(liqoctl), "kubectl-") {
		liqoctl = strings.ReplaceAll(filepath.Base(liqoctl), "-", " ")
		liqoctl = strings.ReplaceAll(liqoctl, "_", "-")
	}

	return liqoctl
}
