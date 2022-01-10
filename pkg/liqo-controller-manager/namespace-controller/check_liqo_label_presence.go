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

package namespacectrl

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// Checks if the Namespace which triggers an Event, contains liqoLabel.
func isLiqoEnabledLabelPresent(labels map[string]string) bool {
	value, ok := labels[liqoconst.EnablingLiqoLabel]
	return ok && value == liqoconst.EnablingLiqoLabelValue
}

// Events not filtered:
// 1 -- liqoLabel is added to the Namespace.
// 2 -- liqoLabel is removed from the Namespace.
// 3 -- create a Namespace with liqoLabel.
func manageLabelPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// if liqoLabel is added  ||  if liqoLabel is removed.
			return (!isLiqoEnabledLabelPresent(e.ObjectOld.GetLabels()) && isLiqoEnabledLabelPresent(e.ObjectNew.GetLabels())) ||
				(isLiqoEnabledLabelPresent(e.ObjectOld.GetLabels()) && !isLiqoEnabledLabelPresent(e.ObjectNew.GetLabels()))
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return isLiqoEnabledLabelPresent(e.Object.GetLabels())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
}
