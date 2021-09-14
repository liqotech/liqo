// Copyright 2019-2021 The Liqo Authors
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

package nodeselector

import corev1 "k8s.io/api/core/v1"

// GetMergedNodeSelector gets two NodeSelector and merges them together.
// Every NodeSelectorTerm of the first Selector must be merged with all the NodeSelectorTerms of the second one:
// n NodeSelectorTerms of the first Selector.
// m NodeSelectorTerms of the second Selector.
// m * n NodeSelectorTerms of the final merged Selector.
func GetMergedNodeSelector(first *corev1.NodeSelector,
	second *corev1.NodeSelector) corev1.NodeSelector {
	mergedNodeSelector := corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}}
	for i := range first.NodeSelectorTerms {
		for j := range second.NodeSelectorTerms {
			newMatchExpressions := second.NodeSelectorTerms[j].MatchExpressions
			newMatchExpressions = append(newMatchExpressions,
				first.NodeSelectorTerms[i].MatchExpressions...)
			mergedNodeSelector.NodeSelectorTerms = append(mergedNodeSelector.NodeSelectorTerms, corev1.NodeSelectorTerm{
				MatchExpressions: newMatchExpressions,
			})
		}
	}
	return mergedNodeSelector
}
