// Copyright 2019-2023 The Liqo Authors
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

package virtualnodectrl

import (
	"context"

	"k8s.io/klog/v2"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
)

func (r *VirtualNodeReconciler) ensureVirtualNodeFinalizerPresence(ctx context.Context, virtualNode *virtualkubeletv1alpha1.VirtualNode) error {
	ctrlutil.AddFinalizer(virtualNode, virtualNodeControllerFinalizer)
	if err := r.Client.Update(ctx, virtualNode); err != nil {
		klog.Errorf(" %s --> Unable to add the finalizer to the virtual-node", err)
		return err
	}
	return nil
}

func (r *VirtualNodeReconciler) removeVirtualNodeFinalizer(ctx context.Context, virtualNode *virtualkubeletv1alpha1.VirtualNode) error {
	var err error
	ctrlutil.RemoveFinalizer(virtualNode, virtualNodeControllerFinalizer)
	if err = r.Client.Update(ctx, virtualNode); err != nil {
		return err
	}
	return err
}
