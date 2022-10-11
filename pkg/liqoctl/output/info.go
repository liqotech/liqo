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
	"strings"

	"github.com/pterm/pterm"
)

// Section is a section of the output.
type Section interface {
	AddSection(title string) Section
	AddSectionWithDetail(title, detail string) Section
	AddEntry(key string, values ...string) Section
	SprintForBox(printer *Printer) string
}

// entry is a entry of the output.
type entry struct {
	key    string
	values []string
}

// section is a section of the output.
type section struct {
	title, detail string
	sections      []*section
	entries       []*entry
}

// NewRootSection create a new Section.
func NewRootSection() Section {
	return &section{}
}

// AddSection add a new Section.
func (s *section) AddSection(title string) Section {
	return s.AddSectionWithDetail(title, "")
}

// AddSectionWithDetail add a new Section.
func (s *section) AddSectionWithDetail(title, detail string) Section {
	section := &section{title: title, detail: detail}
	s.sections = append(s.sections, section)
	return section
}

// AddEntry add a new entry.
func (s *section) AddEntry(key string, values ...string) Section {
	s.entries = append(s.entries, &entry{key: key, values: values})
	return s
}

// SprintForBox print the section for box.
func (s *section) SprintForBox(printer *Printer) string {
	s.print(-1, printer)
	return printer.BulletListSprintForBox()
}

// String return the string representation of the section.
func (s *section) String() string {
	if s.detail != "" {
		return pterm.Sprintf(
			"%s - %s",
			StatusSectionStyle.Sprint(s.title),
			StatusInfoStyle.Sprint(s.detail),
		)
	}
	return StatusSectionStyle.Sprint(s.title)
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
		printer.BulletListAddItemWithoutBullet(pterm.Sprintf("%s", e.key),
			level,
		)
	case 1:
		printer.BulletListAddItemWithoutBullet(pterm.Sprintf("%s: %s%s",
			e.key,
			strings.Repeat(" ", longestKey-len(e.key)),
			StatusDataStyle.Sprint(e.values[0]),
		),
			level,
		)
	default:
		printer.BulletListAddItemWithoutBullet(
			pterm.Sprintf("%s:", pterm.Sprint(e.key)),
			level,
		)
		for _, v := range e.values {
			printer.BulletListAddItemWithBullet(
				StatusDataStyle.Sprint(v),
				level,
			)
		}
	}
}
