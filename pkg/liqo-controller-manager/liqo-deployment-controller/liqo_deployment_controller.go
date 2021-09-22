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

package liqodeploymentctrl

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const (
	defaultLabel = "kubernetes.io/hostname"
)

// LiqoDeploymentReconciler
type LiqoDeploymentReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	SelectedClusters map[string]struct{}
}

// Reconcile
func (r *LiqoDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Get the LiqoDeployment resource.
	liqoDeployment := &offv1alpha1.LiqoDeployment{}
	if err := r.Get(ctx, req.NamespacedName, liqoDeployment); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no LiqoDeployment resource called '%s' in the cluster", req.Name)
			return ctrl.Result{}, nil
		}
		klog.Errorf("%s --> Unable to get the LiqoDeployment '%s'", err, req.Name)
		return ctrl.Result{}, err
	}

	original := liqoDeployment.DeepCopy()
	// If no label are specified, the default one is chosen.
	if len(liqoDeployment.Spec.GroupByLabels) == 0 {
		liqoDeployment.Spec.GroupByLabels = append(liqoDeployment.Spec.GroupByLabels, defaultLabel)
	}

	// Get the NamespaceOffloading resource in the LiqoDeployment namespace.
	// If there is no NamespaceOffloading resource in that namespace, it is an error. It is not possible
	// to replicate deployment inside remote clusters, because there are no remote namespaces.
	namespaceOffloading := &offv1alpha1.NamespaceOffloading{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: req.Namespace,
		Name:      liqoconst.DefaultNamespaceOffloadingName,
	}, namespaceOffloading); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Errorf("%s -> There is no NamespaceOffloading resource in the namespace '%s'.", err, req.Namespace)
		} else {
			klog.Errorf("%s -> Unable to get the NamespaceOffloading resource in the namespace '%s'.", err, req.Namespace)
		}
		return ctrl.Result{}, err
	}

	// Merge the NodeSelector specified in the LiqoDeployment with the one specified in the NamespaceOffloading
	// If the NodeSelector is not specified in the LiqoDeployment resource, the resulting Selector will be equal to
	// the NamespaceOffloading Selector.
	clusterFilter := getClusterFilter(namespaceOffloading, liqoDeployment)

	// Check which nodes remain after the selection and which combination of generationLabels
	// are present between the various virtual nodes.
	if err := r.checkCompatibleVirtualNodes(ctx, &clusterFilter, liqoDeployment); err != nil {
		return ctrl.Result{}, err
	}

	// Create replicated deployment.
	creationNotCompleted := r.enforceDeploymentReplicas(ctx, liqoDeployment)

	// Check if deployment in the status map are still required.
	deletionNotCompleted := r.searchUnnecessaryDeploymentReplicas(liqoDeployment, ctx)

	if err := r.Patch(ctx, liqoDeployment, client.MergeFrom(original)); err != nil {
		klog.Errorf("%s -> Unable to patch the LiqoDepoyment '%s' Status", err, liqoDeployment.Name)
		return ctrl.Result{}, err
	}

	// If there was an error during the creation or deletion phase.
	if creationNotCompleted || deletionNotCompleted {
		return ctrl.Result{}, fmt.Errorf("waiting for the deletion of some deployment replicas")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager.
func (r *LiqoDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&offv1alpha1.LiqoDeployment{}).
		Owns(&appsv1.Deployment{}).
		Watches(&source.Kind{Type: &corev1.Node{}}, getVirtualNodeEventHandler(r.Client)).
		Complete(r)
}
