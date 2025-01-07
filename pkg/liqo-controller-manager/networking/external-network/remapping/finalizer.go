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

package remapping

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
)

const (
	// ipMappingControllerFinalizer is the finalizer added to IP resources (related to mapping) to allow the controller to clean up.
	ipMappingControllerFinalizer = "ipmapping-nat-controller.liqo.io/finalizer"
)

func (r *IPReconciler) ensureIPMappingFinalizerPresence(
	ctx context.Context, ip *ipamv1alpha1.IP) error {
	controllerutil.AddFinalizer(ip, ipMappingControllerFinalizer)
	return r.Client.Update(ctx, ip)
}

func (r *IPReconciler) ensureIPMappingFinalizerAbsence(
	ctx context.Context, ip *ipamv1alpha1.IP) error {
	controllerutil.RemoveFinalizer(ip, ipMappingControllerFinalizer)
	return r.Client.Update(ctx, ip)
}
