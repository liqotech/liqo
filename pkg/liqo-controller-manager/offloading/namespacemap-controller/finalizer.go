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

package namespacemapctrl

import (
	"context"

	"k8s.io/klog/v2"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

const (
	// NamespaceMapControllerFinalizer is the finalizer added to NamespaceMaps to ensure graceful termination.
	NamespaceMapControllerFinalizer = "namespacemap-controller.liqo.io/finalizer"
)

// SetNamespaceMapControllerFinalizer adds NamespaceMapControllerFinalizer to
// a NamespaceMap, if it is not already there.
func (r *NamespaceMapReconciler) SetNamespaceMapControllerFinalizer(ctx context.Context, nm *offloadingv1beta1.NamespaceMap) error {
	if ctrlutils.ContainsFinalizer(nm, NamespaceMapControllerFinalizer) {
		return nil
	}

	ctrlutils.AddFinalizer(nm, NamespaceMapControllerFinalizer)
	if err := r.Update(ctx, nm); err != nil {
		klog.Errorf("Failed to add finalizer to the NamespaceMap %q: %v", klog.KObj(nm), err)
		return err
	}

	klog.Infof("Finalizer correctly added to NamespaceMap %q", klog.KObj(nm))
	return nil
}

// RemoveNamespaceMapControllerFinalizer remove the NamespaceMapController finalizer.
func (r *NamespaceMapReconciler) RemoveNamespaceMapControllerFinalizer(ctx context.Context, nm *offloadingv1beta1.NamespaceMap) error {
	if !ctrlutils.ContainsFinalizer(nm, NamespaceMapControllerFinalizer) {
		return nil
	}

	ctrlutils.RemoveFinalizer(nm, NamespaceMapControllerFinalizer)
	if err := r.Update(ctx, nm); err != nil {
		klog.Errorf("Failed to remove finalizer from NamespaceMap %q: %v", klog.KObj(nm), err)
		return err
	}

	klog.Infof("Finalizer correctly removed from NamespaceMap %q", klog.KObj(nm))
	return nil
}
