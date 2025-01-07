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

package identitycreatorcontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	klog "k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// NewIdentityCreatorReconciler returns a new IdentityCreatorReconciler.
func NewIdentityCreatorReconciler(cl client.Client, s *runtime.Scheme,
	recorder record.EventRecorder, liqoNamespace string,
	localClusterID liqov1beta1.ClusterID) *IdentityCreatorReconciler {
	return &IdentityCreatorReconciler{
		Client: cl,
		Scheme: s,

		eventRecorder: recorder,

		liqoNamespace:  liqoNamespace,
		localClusterID: localClusterID,
	}
}

// IdentityCreatorReconciler reconciles a ResourceSliceReconciler object.
type IdentityCreatorReconciler struct {
	client.Client
	*runtime.Scheme

	eventRecorder record.EventRecorder

	liqoNamespace  string
	localClusterID liqov1beta1.ClusterID
}

// cluster-role
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=resourceslices;resourceslices/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=identities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile local ResourceSlice resources.
func (r *IdentityCreatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var resourceSlice authv1beta1.ResourceSlice
	if err := r.Get(ctx, req.NamespacedName, &resourceSlice); err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("resourceSlice %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("unable to get ResourceSlice %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Delete identity if ResourceSlice Authentication is Denied.
	if authenticationDenied(&resourceSlice) {
		identity := forge.Identity(forge.ResourceSliceIdentityName(&resourceSlice), resourceSlice.Namespace)
		if err := r.Delete(ctx, identity); err != nil && !errors.IsNotFound(err) {
			klog.Errorf("unable to delete Identity %q: %v", identity.Name, err)
			return ctrl.Result{}, err
		} else if err == nil {
			klog.Infof("Deleted Identity associated to ResourceSlice %q", req.NamespacedName)
			r.eventRecorder.Event(&resourceSlice, corev1.EventTypeNormal, "IdentityDeleted", "Identity deleted")
		}
		return ctrl.Result{}, nil
	}

	if resourceSlice.Spec.ProviderClusterID == nil {
		err := fmt.Errorf("ResourceSlice %q has no ProviderClusterID", req.NamespacedName)
		klog.Error(err)
		return ctrl.Result{}, err
	}

	if resourceSlice.Status.AuthParams == nil {
		err := fmt.Errorf("ResourceSlice %q has no AuthParams", req.NamespacedName)
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// Create or update the Identity resource.
	identity := forge.Identity(forge.ResourceSliceIdentityName(&resourceSlice), resourceSlice.Namespace)
	if _, err := resource.CreateOrUpdate(ctx, r.Client, identity, func() error {
		forge.MutateIdentity(identity, *resourceSlice.Spec.ProviderClusterID, authv1beta1.ResourceSliceIdentityType,
			resourceSlice.Status.AuthParams, nil)
		if identity.Labels == nil {
			identity.Labels = make(map[string]string)
		}
		identity.Labels[consts.ResourceSliceNameLabelKey] = resourceSlice.Name
		return controllerutil.SetControllerReference(&resourceSlice, identity, r.Scheme)
	}); err != nil {
		klog.Errorf("unable to create or update Identity %q: %v", identity.Name, err)
		return ctrl.Result{}, err
	}

	klog.Infof("Ensured Identity associated to ResourceSlice %q", req.NamespacedName)
	r.eventRecorder.Event(&resourceSlice, corev1.EventTypeNormal, "IdentityEnsured", "Identity ensured")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IdentityCreatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// generate the predicate to filter just the ResourceSlices created by the local cluster checking crdReplicator labels
	localResSliceFilter, err := predicate.LabelSelectorPredicate(reflection.LocalResourcesLabelSelector())
	if err != nil {
		klog.Error(err)
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlIdentityCreator).
		For(&authv1beta1.ResourceSlice{}, builder.WithPredicates(predicate.And(localResSliceFilter, withAuthCondition()))).
		Owns(&authv1beta1.Identity{}).
		Complete(r)
}

func withAuthCondition() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		rs, ok := obj.(*authv1beta1.ResourceSlice)
		if !ok {
			return false
		}

		return authentication.GetCondition(rs, authv1beta1.ResourceSliceConditionTypeAuthentication) != nil
	})
}

func authenticationDenied(resourceSlice *authv1beta1.ResourceSlice) bool {
	authCondition := authentication.GetCondition(resourceSlice, authv1beta1.ResourceSliceConditionTypeAuthentication)

	return authCondition.Status == authv1beta1.ResourceSliceConditionDenied
}
