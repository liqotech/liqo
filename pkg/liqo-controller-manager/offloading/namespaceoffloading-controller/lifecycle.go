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

	"k8s.io/klog/v2"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

func (r *NamespaceOffloadingReconciler) deletionLogic(ctx context.Context,
	nsoff *offloadingv1beta1.NamespaceOffloading, clusterIDMap map[string]*offloadingv1beta1.NamespaceMap) error {
	klog.Infof("The NamespaceOffloading of the namespace %q is requested to be deleted", nsoff.Namespace)
	// 1 - remove Liqo scheduling label from the associated namespace.
	if err := r.enforceSchedulingLabelAbsence(ctx, nsoff.Namespace); err != nil {
		return err
	}
	// 2 - remove the involved DesiredMapping from the NamespaceMap.
	if err := removeDesiredMappings(ctx, r.Client, nsoff.Namespace, clusterIDMap); err != nil {
		return err
	}
	// 3 - check if all remote namespaces associated with this NamespaceOffloading resource are really deleted.
	if len(nsoff.Status.RemoteNamespacesConditions) != 0 {
		err := fmt.Errorf("waiting for remote namespaces deletion")
		klog.V(4).Infof("NamespaceOffloading %q terminating: %v", klog.KObj(nsoff), err)
		return err
	}
	// 4 - remove NamespaceOffloading controller finalizer; all remote namespaces associated with this resource
	// have been deleted.
	ctrlutils.RemoveFinalizer(nsoff, namespaceOffloadingControllerFinalizer)
	if err := r.Update(ctx, nsoff); err != nil {
		return fmt.Errorf("failed to remove finalizer from NamespaceOffloading %q: %w", klog.KObj(nsoff), err)
	}

	klog.Infof("Finalizer correctly removed from NamespaceOffloading %q", klog.KObj(nsoff))
	return nil
}

func (r *NamespaceOffloadingReconciler) enforceFinalizerPresence(ctx context.Context, nsoff *offloadingv1beta1.NamespaceOffloading) error {
	if ctrlutils.ContainsFinalizer(nsoff, namespaceOffloadingControllerFinalizer) {
		return nil
	}

	ctrlutils.AddFinalizer(nsoff, namespaceOffloadingControllerFinalizer)
	if err := r.Update(ctx, nsoff); err != nil {
		return fmt.Errorf("failed to add finalizer to NamespaceOffloading %q: %w", klog.KObj(nsoff), err)
	}

	klog.Infof("Finalizer correctly added to NamespaceOffloading %q", klog.KObj(nsoff))
	return nil
}
