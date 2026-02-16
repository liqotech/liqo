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
// https://github.com/spf13/cobra/blob/main/doc/md_docs.go

// Package docs contains the functions to generate the markdown documentation for provided commands.
package docs

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// GenCommandPage generates the markdown documentation for the provided command and its children.
// If genChildrenPages is true, it generates a separate markdown file for each child command.
// If level is greater than 1, it generates a second level paragraph.
func GenCommandPage(cmd *cobra.Command, dir string, genChildrenPages bool, level int) error {
	isParent := level <= 1

	pageAddlContent := new(bytes.Buffer)
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}

		if genChildrenPages {
			if err := GenCommandPage(c, dir, false, level+1); err != nil {
				return err
			}
		} else {
			if err := GenCommandMarkdown(c, pageAddlContent, false); err != nil {
				return err
			}
		}
	}

	// Create the directory if it does not exist
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}

	basename := strings.ReplaceAll(cmd.CommandPath(), " ", "_") + markdownExtension
	filename := filepath.Clean(filepath.Join(dir, basename))

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := GenCommandMarkdown(cmd, f, isParent); err != nil {
		return err
	}

	if pageAddlContent.Len() > 0 {
		if _, err := f.WriteString(pageAddlContent.String()); err != nil {
			return err
		}
	}
	return nil
}

// GenCommandMarkdown generates the markdown for the given command.
func GenCommandMarkdown(cmd *cobra.Command, w io.Writer, isParent bool) error {
	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultHelpFlag()

	buf := new(bytes.Buffer)
	name := cmd.CommandPath()

	// If it is not parent command, we use a second level paragraph.
	if !isParent {
		buf.WriteString("#")
	}
	buf.WriteString("# " + name + "\n\n")
	buf.WriteString(replaceExecutableName(cmd.Short) + "\n\n")

	if isParent {
		buf.WriteString("## Description\n\n")
	}

	examples := ""
	if cmd.Long != "" {
		longHelp := formatLongHelp(cmd.Long)
		// Split example from command long description
		splittedLongHelp := strings.Split(longHelp, "Examples:")

		buf.WriteString("### Synopsis\n\n")
		buf.WriteString(splittedLongHelp[0] + "\n\n")

		// We found some examples in the description
		if len(splittedLongHelp) > 1 {
			examples = splittedLongHelp[1]
		}
	}

	if cmd.Runnable() {
		fmt.Fprintf(buf, "```\n%s\n```\n\n", cmd.UseLine())

		if examples != "" {
			buf.WriteString("### Examples\n\n")
			buf.WriteString(formatExamples(examples))
		}

		if err := printOptions(buf, cmd); err != nil {
			return err
		}
	}

	_, err := buf.WriteTo(w)
	return err
}

func printOptions(buf *bytes.Buffer, cmd *cobra.Command) error {
	flags := cmd.NonInheritedFlags()
	flags.SetOutput(buf)
	if flags.HasAvailableFlags() {
		buf.WriteString("### Options\n")
		writeFlags(buf, flags)
		buf.WriteString("\n")
	}

	parentFlags := cmd.InheritedFlags()
	parentFlags.SetOutput(buf)
	if parentFlags.HasAvailableFlags() {
		buf.WriteString("### Global options\n\n")
		writeFlags(buf, parentFlags)
	}
	return nil
}
