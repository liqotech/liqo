// Copyright 2019-2026 The Liqo Authors
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
// Portions of this file are derived from the spf13/cobra project,
// which is licensed under the Apache License, Version 2.0.
// https://github.com/spf13/cobra/blob/main/doc/util.go

package docs

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

const markdownExtension = ".md"
const customExecutableName = "liqoctl"
const stringType = "string"
const warningPrefix = "warning:"

var originalExecutable = os.Args[0]

func isZero(s string) bool {
	return s == "" || s == "0" || s == "false" || s == "[]"
}

func formatExamples(examples string) string {
	lines := []string{}

	inBlock := false
	for i, line := range strings.Split(examples, "\n") {
		switch {
		case strings.HasPrefix(strings.TrimSpace(line), "$"), strings.HasPrefix(strings.TrimSpace(line), "-"):
			if !inBlock {
				lines = append(lines, "```bash")
			}
			lines = append(lines, line)
			inBlock = true
		case i == 0:
			lines = append(lines, line)
		default:
			if inBlock {
				lines = append(lines, "```")
				inBlock = false
			}
			lines = append(lines, fmt.Sprintf("\n%s\n", line))
		}
	}

	if inBlock {
		lines = append(lines, "```\n")
	}

	return strings.Join(lines, "\n")
}

func writeFlags(buf *bytes.Buffer, flags *pflag.FlagSet) {
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}

		line := ""
		if flag.Name == "help" {
			return
		}

		if flag.Shorthand != "" && flag.ShorthandDeprecated == "" {
			line = fmt.Sprintf("`-%s`, `--%s`", flag.Shorthand, flag.Name)
		} else {
			line = fmt.Sprintf("`--%s`", flag.Name)
		}

		vartype, usage := pflag.UnquoteUsage(flag)
		if vartype != "" {
			line += " _" + vartype + "_:"
		}

		if flag.NoOptDefVal != "" {
			switch flag.Value.Type() {
			case stringType:
				line += fmt.Sprintf("[=%q]", flag.NoOptDefVal)
			case "bool":
				if flag.NoOptDefVal != "true" {
					line += fmt.Sprintf("[=%s]", flag.NoOptDefVal)
				}
			case "count":
				if flag.NoOptDefVal != "+1" {
					line += fmt.Sprintf("[=%s]", flag.NoOptDefVal)
				}
			default:
				line += fmt.Sprintf("[=%s]", flag.NoOptDefVal)
			}
		}

		line += fmt.Sprintf("\n\n>%s", usage)

		if !isZero(flag.DefValue) {
			if flag.Value.Type() == stringType {
				line += fmt.Sprintf(" **(default %q)**", flag.DefValue)
			} else {
				line += fmt.Sprintf(" **(default %s)**", flag.DefValue)
			}
		}
		if flag.Deprecated != "" {
			line += fmt.Sprintf(" (DEPRECATED: %s)", flag.Deprecated)
		}

		buf.WriteString(line + "\n\n")
	})
}

func formatLongHelp(text string) string {
	return replaceWarnings(replaceExecutableName(text))
}

func replaceExecutableName(text string) string {
	return strings.ReplaceAll(text, originalExecutable, customExecutableName)
}

func replaceWarnings(text string) string {
	var result strings.Builder
	lines := strings.Split(text, "\n")
	inWarningBlock := false
	warningLen := len(warningPrefix)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch {
		case len(trimmed) > warningLen && strings.EqualFold(trimmed[:warningLen], "warning:"):
			if !inWarningBlock {
				result.WriteString("```{warning}\n")
				inWarningBlock = true
			}
			result.WriteString(strings.TrimPrefix(trimmed[warningLen:], "warning:") + "\n")
		case inWarningBlock && trimmed == "":
			result.WriteString("```\n")
			inWarningBlock = false
		default:
			result.WriteString(line + "\n")
		}
	}

	if inWarningBlock {
		result.WriteString("```\n")
	}

	return result.String()
}
