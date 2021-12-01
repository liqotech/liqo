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

package liqodeploymentctrl

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	appsv1alpha1 "github.com/liqotech/liqo/apis/apps/v1alpha1"
)

// Reconciler reconciles LiqoDeployment resources.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.liqo.io,resources=liqodeployments,verbs=get;list;watch;update;patch;delete

// Reconcile checks the LiqoDeployment Spec and creates necessary deployment replicas, updating the LiqoDeployment status.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	nsName := req.NamespacedName
	klog.V(4).Infof("reconcile liqodeployment %s", nsName)

	liqoDeployment := appsv1alpha1.LiqoDeployment{}
	if err := r.Get(ctx, nsName, &liqoDeployment); err != nil {
		err = client.IgnoreNotFound(err)
		if err == nil {
			klog.V(4).Infof("skip: liqodeployment %s not found", nsName)
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets when to reconcile the controller.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.LiqoDeployment{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}
