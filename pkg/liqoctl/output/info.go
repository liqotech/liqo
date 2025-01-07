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
	"strings"

	"github.com/pterm/pterm"
)

// Section is a section of the output.
type Section interface {
	AddSection(title string) Section
	AddSectionSuccess(title string) Section
	AddSectionFailure(title string) Section
	AddSectionInfo(title string) Section
	AddSectionWithDetail(title, detail string) Section
	AddEntry(key string, values ...string) Section
	AddEntryWarning(key string, values ...string) Section
	AddEntryWithoutStyle(key, value string) Section
	SprintForBox(printer *Printer) string
}

// entry is a entry of the output.
type entry struct {
	key    string
	values []string
	style  *pterm.Style
}

type sectionVariant string

const (
	success sectionVariant = "success"
	failure sectionVariant = "failure"
	info    sectionVariant = "info"
)

// section is a section of the output.
type section struct {
	title, detail string
	sections      []*section
	entries       []*entry
	variant       sectionVariant
}

// NewRootSection create a new Section.
func NewRootSection() Section {
	return &section{}
}

// AddSection add a new Section.
func (s *section) AddSection(title string) Section {
	return s.AddSectionWithDetail(title, "")
}

// AddSectionSuccess add a new Success Section.
func (s *section) AddSectionSuccess(title string) Section {
	section := &section{title: title, detail: "", variant: success}
	s.sections = append(s.sections, section)
	return section
}

// AddSectionFailure add a new Failure Section.
func (s *section) AddSectionFailure(title string) Section {
	section := &section{title: title, detail: "", variant: failure}
	s.sections = append(s.sections, section)
	return section
}

// AddSectionInfo add a new Info Section.
func (s *section) AddSectionInfo(title string) Section {
	section := &section{title: title, detail: "", variant: info}
	s.sections = append(s.sections, section)
	return section
}

// AddSectionWithDetail add a new Section.
func (s *section) AddSectionWithDetail(title, detail string) Section {
	section := &section{title: title, detail: detail}
	s.sections = append(s.sections, section)
	return section
}

// AddEntry add a new entry.
func (s *section) AddEntry(key string, values ...string) Section {
	s.entries = append(s.entries, &entry{key: key, values: values, style: StatusDataStyle})
	return s
}

// AddEntryWarning add a new entry with warning style.
func (s *section) AddEntryWarning(key string, values ...string) Section {
	s.entries = append(s.entries, &entry{key: key, values: values, style: StatusWarningStyle})
	return s
}

// AddEntryWithoutStyle add a new entry without style.
func (s *section) AddEntryWithoutStyle(key, value string) Section {
	s.entries = append(s.entries, &entry{key: key, values: []string{value}, style: pterm.NewStyle(pterm.FgDefault)})
	return s
}

// SprintForBox print the section for box.
func (s *section) SprintForBox(printer *Printer) string {
	s.print(-1, printer)
	return printer.BulletListSprintForBox()
}

// String return the string representation of the section.
func (s *section) String() string {
	sectionStyle := StatusSectionStyle
	switch s.variant {
	case success:
		sectionStyle = StatusSectionSuccessStyle
	case failure:
		sectionStyle = StatusSectionFailureStyle
	case info:
		sectionStyle = StatusSectionInfoStyle
	}

	if s.detail != "" {
		return pterm.Sprintf(
			"%s - %s",
			sectionStyle.Sprint(s.title),
			StatusInfoStyle.Sprint(s.detail),
		)
	}
	return sectionStyle.Sprint(s.title)
}

// print print the section.
func (s *section) print(level int, printer *Printer) {
	if level >= 0 {
		printer.BulletListAddItemWithoutBullet(s.String(), level)
	}
	for _, entry := range s.entries {
		entry.print(level+1, printer, longestEntryKey(s.entries))
	}
	for _, section := range s.sections {
		section.print(level+1, printer)
	}
}

// longestEntryKey return the longest key length of the entries.
func longestEntryKey(entries []*entry) int {
	longest := 0
	for _, entry := range entries {
		if len(entry.key) > longest {
			longest = len(entry.key)
		}
	}
	return longest
}

// print print the entry.
func (e *entry) print(level int, printer *Printer, longestKey int) {
	switch len(e.values) {
	case 0:
		printer.BulletListAddItemWithoutBullet(pterm.Bold.Sprint(e.key),
			level,
		)
	case 1:
		printer.BulletListAddItemWithoutBullet(pterm.Sprintf("%s: %s%s",
			pterm.Sprint(e.key),
			strings.Repeat(" ", longestKey-len(e.key)),
			e.style.Sprint(e.values[0]),
		),
			level,
		)
	default:
		printer.BulletListAddItemWithoutBullet(
			pterm.Sprint(e.key),
			level,
		)
		for _, v := range e.values {
			printer.BulletListAddItemWithBullet(
				e.style.Sprint(v),
				level,
			)
		}
	}
}
