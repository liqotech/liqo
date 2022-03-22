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
	"fmt"
	"strings"
)

// InfoData implements the InfoInterface interface.
// holds information about a field using a key/value pair.
type InfoData struct {
	key   string
	value []string
}

// StringIndented return the  String() output but with indentation.
func (id *InfoData) StringIndented(indent int) string {
	indentation := strings.Repeat("\t", indent+1)
	if len(id.value) == 1 {
		return fmt.Sprintf("%s: %s%s%s", id.key, byellow, id.value[0], reset)
	}
	msg := fmt.Sprintf("%s:", id.key)
	for _, v := range id.value {
		msg += fmt.Sprintf("\n%s- %s%s%s", indentation, byellow, v, reset)
	}
	return msg
}

// StringNoColor return the  String() output but without colors.
func (id *InfoData) StringNoColor() string {
	return fmt.Sprintf("%s: %s", id.key, id.value)
}

// InfoNode is a node used to link objects implementing InfoInterface in a tree data-structure.
type InfoNode struct {
	title, detail string
	nextNodes     []*InfoNode
	data          []InfoData
}

func (in *InfoNode) String() string {
	if in.detail != "" {
		return fmt.Sprintf("%s%s%s %s[%s]%s", bpurple, in.title, reset, bcyan, in.detail, reset)
	}
	return fmt.Sprintf("%s%s%s", bpurple, in.title, reset)
}

// StringNoColor return the  String() output but without colors.
func (in *InfoNode) StringNoColor() string {
	if in.detail != "" {
		return fmt.Sprintf("%s [%s]", in.title, in.detail)
	}
	return in.title
}

// newRootInfoNode init first InfoLayer inserting a InfoSection.
func newRootInfoNode(sectionTitle string) InfoNode {
	return InfoNode{
		title: sectionTitle,
	}
}

// deepPrintInfo print all InfoNode recursively.
// It is a deepPrintInfoRecursive.
func deepPrintInfo(in *InfoNode) string {
	var msg string
	deepPrintInfoRecursive(in, &msg, 0)
	return msg
}

// deepPrintInfoRecursve print all InfoNode recursively.
func deepPrintInfoRecursive(in *InfoNode, msg *string, nodeLevel int) {
	indentation := strings.Repeat("\t", nodeLevel)

	if nodeLevel > 0 {
		*msg += fmt.Sprintf("%s%s\n", indentation, in.String())
	} else {
		*msg += fmt.Sprintf("%s\n%s\n", in.String(), newSeparator(in.StringNoColor()))
	}

	for i := range in.data {
		*msg += fmt.Sprintf("\t%s%s\n", indentation, in.data[i].StringIndented(nodeLevel+1))
	}
	for i := range in.nextNodes {
		deepPrintInfoRecursive(in.nextNodes[i], msg, nodeLevel+1)
	}
}

// addDataToNode create a new InfoNode and add a new InfoData.
func (in *InfoNode) addDataToNode(key, value string) {
	in.data = append(in.data, InfoData{
		key: key, value: []string{value},
	})
}

//	addDataListToNode create a new InfoNode and add a new InfoData.
func (in *InfoNode) addDataListToNode(key string, value []string) {
	in.data = append(in.data, InfoData{
		key: key, value: value,
	})
}

// addSectionToNode create a new InfoNode and add a new InfoSection.
func (in *InfoNode) addSectionToNode(sectionTitle, detail string) *InfoNode {
	in.nextNodes = append(in.nextNodes, &InfoNode{
		title:  sectionTitle,
		detail: detail,
		data:   []InfoData{},
	})
	return in.nextNodes[len(in.nextNodes)-1]
}

// findNodeByTitle returns the info node with the given title.
func findNodeByTitle(infoNodes []*InfoNode, title string) *InfoNode {
	for i := range infoNodes {
		if infoNodes[i].title == title {
			return infoNodes[i]
		}
	}
	return nil
}
