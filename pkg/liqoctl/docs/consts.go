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

package docs

const (
	// ShortHelp contains the short help string for liqoctl docs command.
	ShortHelp = "Generate documentation as markdown"
	// LongHelp contains the Long help string for liqoctl docs command.
	LongHelp = `
Generate documentation files for liqoctl.

This command can generate documentation for liqoctl in the following formats:
- Markdown

$ liqoctl docs --dir path-to-desired-folder
`
	// UseCommand contains the name of the command.
	UseCommand = "docs"
	// OutputDir contains the name of dir flag.
	OutputDir = "dir"
	// DocType contains the name of type flag.
	DocType = "type"
	// GenerateHeaders contains the name of generate-headers.
	GenerateHeaders = "generate-headers"
)
