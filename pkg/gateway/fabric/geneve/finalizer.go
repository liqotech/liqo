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

package geneve

import (
	"context"

	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

const (
	// internalnodeControllerFinalizer is the finalizer added to internalnode to allow the controller to clean up.
	internalnodeControllerFinalizer = "internalnode-controller.liqo.io/finalizer"
)

func (r *InternalNodeReconciler) ensureinternalnodeFinalizerPresence(
	ctx context.Context, internalnode *networkingv1beta1.InternalNode) error {
	ctrlutil.AddFinalizer(internalnode, internalnodeControllerFinalizer)
	return r.Client.Update(ctx, internalnode)
}

func (r *InternalNodeReconciler) ensureinternalnodeFinalizerAbsence(
	ctx context.Context, internalnode *networkingv1beta1.InternalNode) error {
	ctrlutil.RemoveFinalizer(internalnode, internalnodeControllerFinalizer)
	return r.Client.Update(ctx, internalnode)
}
