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

package namespaceoffloadingctrl

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

func addLiqoSchedulingLabel(ctx context.Context, c client.Client, namespaceName string) error {
	namespace := &corev1.Namespace{}
	if err := c.Get(ctx, types.NamespacedName{Name: namespaceName}, namespace); err != nil {
		klog.Errorf("%s --> Unable to get the namespace '%s'", err, namespaceName)
		return err
	}

	if namespace.Labels == nil {
		namespace.Labels = map[string]string{}
	}

	if value, ok := namespace.Labels[liqoconst.SchedulingLiqoLabel]; !ok || value != liqoconst.SchedulingLiqoLabelValue {
		namespace.Labels[liqoconst.SchedulingLiqoLabel] = liqoconst.SchedulingLiqoLabelValue
		if err := c.Update(ctx, namespace); err != nil {
			klog.Errorf(" %s --> Unable to add liqo scheduling label to the namespace '%s'", err, namespace.GetName())
			return err
		}
		klog.Infof(" Liqo scheduling label successfully added to the namespace '%s'", namespace.GetName())
	}
	return nil
}

func removeLiqoSchedulingLabel(ctx context.Context, c client.Client, namespaceName string) error {
	namespace := &corev1.Namespace{}
	if err := c.Get(ctx, types.NamespacedName{Name: namespaceName}, namespace); err != nil {
		klog.Errorf("%s --> Unable to get the namespace '%s'", err, namespaceName)
		return err
	}

	if value, ok := namespace.Labels[liqoconst.SchedulingLiqoLabel]; ok && value == liqoconst.SchedulingLiqoLabelValue {
		delete(namespace.Labels, liqoconst.SchedulingLiqoLabel)
		if err := c.Update(ctx, namespace); err != nil {
			klog.Errorf(" %s --> Unable to remove Liqo scheduling label from the namespace '%s'", err, namespace.GetName())
			return err
		}
		klog.Infof(" Liqo scheduling label successfully removed from the namespace '%s'", namespace.GetName())
	}
	return nil
}
