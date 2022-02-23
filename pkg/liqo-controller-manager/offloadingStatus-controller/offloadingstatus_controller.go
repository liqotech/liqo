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

package offloadingstatuscontroller

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// OffloadingStatusReconciler checks the status of all remote namespaces associated with this
// namespaceOffloading Resource, and sets the global offloading status in according to the feedbacks received
// from all remote clusters.
type OffloadingStatusReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	RequeueTime time.Duration
}

// cluster-role
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespaceoffloadings,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=namespacemaps,verbs=get;list;watch;delete

// Controller Ownership:
// --> NamespaceOffloading.Status.RemoteConditions
// --> NamespaceOffloading.Status.OffloadingPhase

// Reconcile sets the NamespaceOffloading Status checking the actual status of all remote Namespace.
// This reconcile is performed every RequeueTime.
func (r *OffloadingStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	namespaceOffloading := &offv1alpha1.NamespaceOffloading{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: req.Namespace,
		Name:      liqoconst.DefaultNamespaceOffloadingName,
	}, namespaceOffloading); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no NamespaceOffloading resource in Namespace '%s'", req.Namespace)
			return ctrl.Result{}, nil
		}
		klog.Errorf("%s --> Unable to get namespaceOffloading for the namespace '%s'", err, req.Namespace)
		return ctrl.Result{}, err
	}

	// Get all local NamespaceMaps in the cluster
	metals := reflection.LocalResourcesLabelSelector()
	selector, err := metav1.LabelSelectorAsSelector(&metals)
	utilruntime.Must(err)

	namespaceMapList := &mapsv1alpha1.NamespaceMapList{}
	if err := r.List(ctx, namespaceMapList, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		klog.Errorf("%s -> unable to get NamespaceMaps", err)
		return ctrl.Result{}, err
	}

	original := namespaceOffloading.DeepCopy()

	ensureRemoteConditionsConsistence(namespaceOffloading, namespaceMapList)

	setRemoteConditionsForEveryCluster(namespaceOffloading, namespaceMapList)

	setNamespaceOffloadingStatus(namespaceOffloading)

	// Patch the status just one time at the end of the logic.
	if err := r.Patch(ctx, namespaceOffloading, client.MergeFrom(original)); err != nil {
		klog.Errorf("%s --> Unable to Patch NamespaceOffloading in the namespace '%s'",
			err, namespaceOffloading.Namespace)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.RequeueTime}, nil
}

// SetupWithManager reconciles when a new NamespaceOffloading is created.
func (r *OffloadingStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&offv1alpha1.NamespaceOffloading{}).
		Complete(r)
}
