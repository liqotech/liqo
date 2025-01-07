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

package namespacemapctrl

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/pkg/consts"
)

// NamespaceMapReconciler creates remote namespaces and updates NamespaceMaps Status.
type NamespaceMapReconciler struct {
	client.Client
}

// cluster-role
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.liqo.io,resources=foreignclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespacemaps,verbs=get;watch;list;update;patch;create;delete
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespacemaps/finalizers,verbs=get;update;patch

// needed to approve the certificates
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests,verbs=get;list;watch
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests/status,verbs=update
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests/approval,verbs=update
// +kubebuilder:rbac:groups=certificates.k8s.io,resourceNames=kubernetes.io/kubelet-serving,resources=signers,verbs=approve
// +kubebuilder:rbac:groups=certificates.k8s.io,resourceNames=beta.eks.amazonaws.com/app-serving,resources=signers,verbs=approve
// +kubebuilder:rbac:groups=certificates.k8s.io,resourceNames=kubernetes.io/kube-apiserver-client,resources=signers,verbs=approve

// Reconcile adds/removes NamespaceMap finalizer, and checks differences
// between DesiredMapping and CurrentMapping in order to create/delete the Namespaces if it is necessary.
func (r *NamespaceMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	namespaceMap := &offloadingv1beta1.NamespaceMap{}
	if err := r.Get(ctx, req.NamespacedName, namespaceMap); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("NamespaceMap %q does not exist anymore", klog.KRef(req.Namespace, req.Name))
			return ctrl.Result{}, nil
		}
		klog.Errorf("Failed to retrieve NamespaceMap %q: %v", klog.KRef(req.Namespace, req.Name), err)
		return ctrl.Result{}, err
	}

	// If the NamespaceMap is requested to be deleted
	if !namespaceMap.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.NamespaceMapDeletionProcess(ctx, namespaceMap)
	}

	// If someone deletes the namespaceMap, then it is necessary to remove all remote namespaces
	// associated with this resource before deleting it, so a finalizer is necessary.
	if err := r.SetNamespaceMapControllerFinalizer(ctx, namespaceMap); err != nil {
		return ctrl.Result{}, err
	}

	// Create/Delete remote Namespaces if it is necessary, according to NamespaceMap status.
	if err := r.EnsureNamespaces(ctx, namespaceMap); err != nil {
		klog.Errorf("Updating remote namespaces: %s", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager monitors only updates on NamespaceMap.
func (r *NamespaceMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	filter, err := predicate.LabelSelectorPredicate(reflection.ReplicatedResourcesLabelSelector())
	utilruntime.Must(err)

	enqueuer := func(_ context.Context, obj client.Object) []reconcile.Request {
		nm, found := obj.GetAnnotations()[consts.RemoteNamespaceManagedByAnnotationKey]
		if !found {
			return nil
		}

		namespace, name, err := cache.SplitMetaNamespaceKey(nm)
		if err != nil {
			klog.Warningf("Failed to retrieve NamespaceMap associated with namespace %q, key: %q", obj.GetName(), nm)
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}}}
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlNamespaceMap).
		For(&offloadingv1beta1.NamespaceMap{}, builder.WithPredicates(filter)).
		// It is not possible to use Owns, since a namespaced object cannot own a non-namespaced one,
		// and cross namespace owners are disallowed by design.
		// https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/.
		Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(enqueuer)).
		Watches(&rbacv1.RoleBinding{}, handler.EnqueueRequestsFromMapFunc(enqueuer)).
		Complete(r)
}
