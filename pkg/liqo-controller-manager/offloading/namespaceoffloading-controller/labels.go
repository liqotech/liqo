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

package nsoffctrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

func (r *NamespaceOffloadingReconciler) enforceSchedulingLabelPresence(ctx context.Context, namespaceName string) error {
	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: namespaceName}, namespace); err != nil {
		return fmt.Errorf("failed to retrieve namespace %q: %w", namespaceName, err)
	}

	if namespace.Labels == nil {
		namespace.Labels = map[string]string{}
	}

	if value, ok := namespace.Labels[liqoconst.SchedulingLiqoLabel]; !ok || value != liqoconst.SchedulingLiqoLabelValue {
		namespace.Labels[liqoconst.SchedulingLiqoLabel] = liqoconst.SchedulingLiqoLabelValue
		if err := r.Update(ctx, namespace); err != nil {
			return fmt.Errorf("failed to add liqo scheduling label to namespace %q: %w", namespace.GetName(), err)
		}

		klog.Infof("Liqo scheduling label successfully added to namespace %q", namespace.GetName())
	}

	return nil
}

func (r *NamespaceOffloadingReconciler) enforceSchedulingLabelAbsence(ctx context.Context, namespaceName string) error {
	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: namespaceName}, namespace); err != nil {
		return fmt.Errorf("failed to retrieve namespace %q: %w", namespaceName, err)
	}

	if value, ok := namespace.Labels[liqoconst.SchedulingLiqoLabel]; ok && value == liqoconst.SchedulingLiqoLabelValue {
		delete(namespace.Labels, liqoconst.SchedulingLiqoLabel)
		if err := r.Update(ctx, namespace); err != nil {
			return fmt.Errorf("failed to remove liqo scheduling label from namespace %q: %w", namespace.GetName(), err)
		}

		klog.Infof("Liqo scheduling label successfully removed from namespace %q", namespace.GetName())
	}

	return nil
}
