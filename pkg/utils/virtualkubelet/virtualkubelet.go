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

package virtualkubelet

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// Lister is an interface for listers.
type Lister[T any] interface {
	List(selector labels.Selector) (ret []T, err error)
}

// List returns a list of NamespacedName objects from the given listers.
func List[T Lister[O], O metav1.Object](listers ...T) ([]any, error) {
	var err error
	objs := make([][]O, len(listers))
	tot := 0
	for i, l := range listers {
		objs[i], err = l.List(labels.Everything())
		if err != nil {
			return nil, err
		}
		tot += len(objs[i])
	}
	list := make([]any, tot)
	n := 0
	for i := range listers {
		for j := range objs[i] {
			list[n] = types.NamespacedName{
				Name:      objs[i][j].GetName(),
				Namespace: objs[i][j].GetNamespace(),
			}
			n++
		}
	}
	return list, nil
}
