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

package route

import (
	"context"

	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

const (
	// internalNodesControllerFinalizer is the finalizer added to internalnode to allow the controller to clean up.
	internalNodesControllerFinalizer = "internalnodes-route.liqo.io/finalizer"
)

func (r *InternalNodeReconciler) enforceInternalNodeFinalizerPresence(
	ctx context.Context, internalNode *networkingv1beta1.InternalNode) error {
	ctrlutil.AddFinalizer(internalNode, internalNodesControllerFinalizer)
	return r.Client.Update(ctx, internalNode)
}

func (r *InternalNodeReconciler) enforceInternalNodeFinalizerAbsence(
	ctx context.Context, internalNode *networkingv1beta1.InternalNode) error {
	ctrlutil.RemoveFinalizer(internalNode, internalNodesControllerFinalizer)
	return r.Client.Update(ctx, internalNode)
}
