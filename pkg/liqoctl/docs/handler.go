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

package docs

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Options encapsulates the arguments of the docs command.
type Options struct {
	Root            *cobra.Command
	Destination     string
	DocTypeString   string
	GenerateHeaders bool
}

// Run implements the docs command.
func (o *Options) Run(_ context.Context) error {
	switch o.DocTypeString {
	case "markdown":
		if o.GenerateHeaders {
			standardLinks := func(s string) string { return s }

			hdrFunc := func(filename string) string {
				base := filepath.Base(filename)
				name := strings.TrimSuffix(base, path.Ext(base))
				caser := cases.Title(language.AmericanEnglish)
				title := caser.String(strings.ReplaceAll(name, "_", " "))
				return fmt.Sprintf("---\ntitle: %q\n---\n\n", title)
			}

			return doc.GenMarkdownTreeCustom(o.Root, o.Destination, hdrFunc, standardLinks)
		}
		return doc.GenMarkdownTree(o.Root, o.Destination)
	case "man":
		manHdr := &doc.GenManHeader{Title: "LIQOCTL", Section: "1"}
		return doc.GenManTree(o.Root, manHdr, o.Destination)
	default:
		return errors.Errorf("unknown doc type %q. Try \"markdown\" or \"man\"", o.DocTypeString)
	}
}
