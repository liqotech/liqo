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

package searchdomainoperator

import (
	"context"
	"net/http"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	discovery "github.com/liqotech/liqo/pkg/discoverymanager"
)

// SearchDomainReconciler is the reconciler manager for SearchDomain resources.
type SearchDomainReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ResyncPeriod time.Duration

	LocalCluster discoveryv1alpha1.ClusterIdentity
	DNSAddress   string

	InsecureTransport *http.Transport
}

// Reconcile reconciles SearchDomain resources.
func (r *SearchDomainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Info("Reconciling SearchDomain " + req.Name)

	sd := discoveryv1alpha1.SearchDomain{}
	if err := r.Get(ctx, req.NamespacedName, &sd); err != nil {
		if k8serrors.IsNotFound(err) {
			// has been deleted
			return ctrl.Result{}, nil
		}
		klog.Error(err, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.ResyncPeriod,
		}, err
	}

	authData, err := loadAuthDataFromDNS(r.DNSAddress, sd.Spec.Domain)
	if err != nil {
		klog.Error(err, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.ResyncPeriod,
		}, err
	}
	discovery.UpdateForeignWAN(ctx, r.InsecureTransport, r.Client, r.LocalCluster, authData, &sd)

	klog.Info("SearchDomain " + req.Name + " successfully reconciled")
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: r.ResyncPeriod,
	}, nil
}

// SetupWithManager assigns the operator to a manager.
func (r *SearchDomainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.SearchDomain{}).
		Complete(r)
}
