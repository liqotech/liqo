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

package namespacectrl

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// NamespaceReconciler covers the case in which the user adds the enabling liqo label to his namespace, and the
// NamespaceOffloading resource associated with that namespace is created, if it is not already there.
type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	nsCtrlAnnotationKey   = "liqo.io/resource-controlled-by"
	nsCtrlAnnotationValue = "This resource is created by the Namespace Controller"
)

// cluster-role
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;watch;list
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespaceoffloadings,verbs=get;watch;list;create;delete

// needed to be granted to other operators
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=advertisements,verbs=delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=create;get;list;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=create;delete;get;update
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests,verbs=create;delete;get;update
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=create;delete;get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=create;delete
// +kubebuilder:rbac:groups="",resources=pods/status,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="",resources=pods/eviction,verbs=create
// +kubebuilder:rbac:groups="",resources=nodes/status,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=create;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=create;delete;get;list;watch

// Reconcile covers the case in which the user adds the enabling liqo label to his namespace, and the
// NamespaceOffloading resource associated with that namespace is created, if it is not already there.
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, req.NamespacedName, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no namespace called '%s' in the cluster", req.Name)
			return ctrl.Result{}, nil
		}
		klog.Errorf("%s --> Unable to get namespace '%s'", err, req.Name)
		return ctrl.Result{}, err
	}

	namespaceOffloading := &offv1alpha1.NamespaceOffloading{}
	var namespaceOffloadingIsPresent bool
	checkPresenceErr := r.Get(ctx, types.NamespacedName{
		Namespace: namespace.Name,
		Name:      liqoconst.DefaultNamespaceOffloadingName,
	}, namespaceOffloading)

	// Check if a NamespaceOffloading resource called "offloading" is present in this Namespace.
	switch {
	case checkPresenceErr == nil:
		namespaceOffloadingIsPresent = true
	case apierrors.IsNotFound(checkPresenceErr):
		namespaceOffloadingIsPresent = false
	default:
		klog.Errorf("%s --> Unable to get NamespaceOffloading for the namespace '%s'",
			checkPresenceErr, req.Name)
		return ctrl.Result{}, checkPresenceErr
	}

	// Check if enabling Liqo Label is added, if there is no NamespaceOffloading
	// resource called "offloading", create it.
	if isLiqoEnabledLabelPresent(namespace.Labels) && !namespaceOffloadingIsPresent {
		if err := r.CreateNamespaceOffloading(ctx, namespace); err != nil {
			return ctrl.Result{}, err
		}
	}

	// If enabling Liqo label is removed, and there is a NamespaceOffloading owned by the controller, delete it
	if !isLiqoEnabledLabelPresent(namespace.Labels) && namespaceOffloadingIsPresent {
		if err := r.DeleteNamespaceOffloadingIfOwned(ctx, namespaceOffloading); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager reconciles only when a Namespace is involved in Liqo logic.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		WithEventFilter(manageLabelPredicate()).
		Complete(r)
}
