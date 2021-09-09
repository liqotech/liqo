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

package namespacemapctrl

import (
	"context"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
)

const (
	namespaceMapControllerFinalizer = "namespacemap-controller.liqo.io/finalizer"
)

// SetNamespaceMapControllerFinalizer adds NamespaceMapControllerFinalizer to
// a NamespaceMap, if it is not already there.
func (r *NamespaceMapReconciler) SetNamespaceMapControllerFinalizer(ctx context.Context,
	nm *mapsv1alpha1.NamespaceMap) error {
	if !ctrlutils.ContainsFinalizer(nm, namespaceMapControllerFinalizer) {
		original := nm.DeepCopy()
		ctrlutils.AddFinalizer(nm, namespaceMapControllerFinalizer)
		if err := r.Patch(ctx, nm, client.MergeFrom(original)); err != nil {
			klog.Errorf("%s --> Unable to add finalizer to the NamespaceMap '%s'", err, nm.GetName())
			return err
		}
		klog.Infof("Finalizer correctly added on NamespaceMap '%s'", nm.GetName())
	}
	return nil
}

// RemoveNamespaceMapControllerFinalizer remove the NamespaceMapController finalizer.
func (r *NamespaceMapReconciler) RemoveNamespaceMapControllerFinalizer(ctx context.Context,
	nm *mapsv1alpha1.NamespaceMap) error {
	original := nm.DeepCopy()
	ctrlutils.RemoveFinalizer(nm, namespaceMapControllerFinalizer)
	// MergeFrom forces the resource patch, without conflicts
	if err := r.Patch(ctx, nm, client.MergeFrom(original)); err != nil {
		klog.Errorf("%s --> Unable to remove '%s' from NamespaceMap '%s'", err, namespaceMapControllerFinalizer, nm.GetName())
		return err
	}
	klog.Infof("Finalizer correctly removed from NamespaceMap '%s'", nm.GetName())
	return nil
}
