// Copyright 2019-2024 The Liqo Authors
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

package remoteresourceslicecontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	klog "k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// NewRemoteResourceSliceReconciler returns a new RemoteResourceSliceReconciler.
func NewRemoteResourceSliceReconciler(cl client.Client, s *runtime.Scheme, config *rest.Config,
	recorder record.EventRecorder, identityProvider identitymanager.IdentityProvider,
	apiServerAddressOverride string, caOverride []byte, trustedCA bool) *RemoteResourceSliceReconciler {
	return &RemoteResourceSliceReconciler{
		Client: cl,
		Scheme: s,
		Config: config,

		eventRecorder:    recorder,
		identityProvider: identityProvider,

		apiServerAddressOverride: apiServerAddressOverride,
		caOverride:               caOverride,
		trustedCA:                trustedCA,

		reconciledClasses: []authv1alpha1.ResourceSliceClass{
			authv1alpha1.ResourceSliceClassDefault,
			authv1alpha1.ResourceSliceClassUnknown,
		},
	}
}

// RemoteResourceSliceReconciler reconciles a ResourceSliceReconciler object.
type RemoteResourceSliceReconciler struct {
	client.Client
	*runtime.Scheme
	Config *rest.Config

	eventRecorder    record.EventRecorder
	identityProvider identitymanager.IdentityProvider

	apiServerAddressOverride string
	caOverride               []byte
	trustedCA                bool

	reconciledClasses []authv1alpha1.ResourceSliceClass
}

// cluster-role
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=resourceslices;resourceslices/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile replicated ResourceSlice resources.
func (r *RemoteResourceSliceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	var resourceSlice authv1alpha1.ResourceSlice
	if err = r.Get(ctx, req.NamespacedName, &resourceSlice); err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("resourceSlice %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("unable to get ResourceSlice %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if resourceSlice.Spec.ConsumerClusterIdentity == nil {
		err = fmt.Errorf("ConsumerClusterIdentity not set")
		klog.Errorf("Unable to ensure the remote certificate for the ResourceSlice %q: %s", req.NamespacedName, err)
		r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "RemoteCertificateFailed", err.Error())
		return ctrl.Result{}, nil
	}

	// check that the CSR is valid

	tenant, err := getters.GetTenantByClusterID(ctx, r.Client, resourceSlice.Spec.ConsumerClusterIdentity.ClusterID)
	if err != nil {
		klog.Errorf("Unable to get the Tenant for the ResourceSlice %q: %s", req.NamespacedName, err)
		r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "TenantNotFound", err.Error())
		return ctrl.Result{}, err
	}

	if err = authentication.CheckCSRForResourceSlice(
		tenant.Spec.PublicKey, &resourceSlice); err != nil {
		klog.Errorf("Invalid CSR for the ResourceSlice %q: %s", req.NamespacedName, err)
		r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "InvalidCSR", err.Error())
		return ctrl.Result{}, nil
	}

	defer func() {
		errDef := r.Client.Status().Update(ctx, &resourceSlice)
		if errDef != nil {
			klog.Errorf("Unable to update the ResourceSlice %q: %s", req.NamespacedName, errDef)
			r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "UpdateFailed", errDef.Error())
			err = errDef
		}

		if err == nil {
			r.eventRecorder.Event(&resourceSlice, corev1.EventTypeNormal, "Reconciled", "ResourceSlice reconciled")
		}
	}()

	if isInResourceClasses(&resourceSlice, r.reconciledClasses...) {
		// TODO: compute the resources
		findOrDefault := func(resource corev1.ResourceName, val resource.Quantity) resource.Quantity {
			if _, ok := resourceSlice.Spec.Resources[resource]; !ok {
				return val
			}
			return resourceSlice.Spec.Resources[resource]
		}
		resourceSlice.Status.Resources = corev1.ResourceList{
			corev1.ResourceCPU:    findOrDefault(corev1.ResourceCPU, resource.MustParse("2")),
			corev1.ResourceMemory: findOrDefault(corev1.ResourceMemory, resource.MustParse("4Gi")),
			corev1.ResourcePods:   findOrDefault(corev1.ResourcePods, resource.MustParse("100")),
		}

		acceptResources(&resourceSlice, r.eventRecorder)
	}
	// TODO: add to status the StorageClasses
	// TODO: add to status the IngressClasses
	// TODO: add to status the LoadBalancerClasses
	// TODO: add to status the NodeLabels

	// forge the AuthParams

	authParams, err := r.identityProvider.ForgeAuthParams(ctx, &identitymanager.SigningRequestOptions{
		Cluster:         resourceSlice.Spec.ConsumerClusterIdentity,
		TenantNamespace: resourceSlice.Namespace,
		IdentityType:    authv1alpha1.ResourceSliceIdentityType,
		Name:            resourceSlice.Name,
		SigningRequest:  resourceSlice.Spec.CSR,

		APIServerAddressOverride: r.apiServerAddressOverride,
		CAOverride:               r.caOverride,
		TrustedCA:                r.trustedCA,
		ResourceSlice:            &resourceSlice,
	})
	if err != nil {
		klog.Errorf("Unable to forge the AuthParams for the ResourceSlice %q: %s", req.NamespacedName, err)
		r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "AuthParamsFailed", err.Error())
		return ctrl.Result{}, err
	}

	resourceSlice.Status.AuthParams = authParams

	// accept the ResourceSlice
	acceptResourceSlice(&resourceSlice, r.eventRecorder)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RemoteResourceSliceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// generate the predicate to filter just the ResourceSlices created by the remote cluster checking crdReplicator labels
	remoteResSliceFilter, err := predicate.LabelSelectorPredicate(reflection.ReplicatedResourcesLabelSelector())
	if err != nil {
		klog.Error(err)
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&authv1alpha1.ResourceSlice{}, builder.WithPredicates(predicate.And(remoteResSliceFilter, withCSR()))).
		Complete(r)
}

func acceptResourceSlice(resourceSlice *authv1alpha1.ResourceSlice, er record.EventRecorder) {
	switch authentication.EnsureCondition(
		resourceSlice,
		authv1alpha1.ResourceSliceConditionTypeAuthentication,
		authv1alpha1.ResourceSliceConditionAccepted,
		"ResourceSliceAuthenticationAccepted",
		"ResourceSlice authentication accepted",
	) {
	case controllerutil.OperationResultNone:
		klog.V(4).Infof("ResourceSlice authentication %q already accepted", resourceSlice.Name)
	case controllerutil.OperationResultUpdated:
		klog.Infof("ResourceSlice authentication %q confirmed", resourceSlice.Name)
		er.Event(resourceSlice, corev1.EventTypeNormal, "ResourceSliceAuthenticationAccepted", "ResourceSlice authentication confirmed")
	case controllerutil.OperationResultCreated:
		klog.Infof("ResourceSlice authentication %q accepted", resourceSlice.Name)
		er.Event(resourceSlice, corev1.EventTypeNormal, "ResourceSliceAuthenticationAccepted", "ResourceSlice authentication accepted")
	default:
		return
	}
}

func isInResourceClasses(resourceSlice *authv1alpha1.ResourceSlice, classes ...authv1alpha1.ResourceSliceClass) bool {
	for _, class := range classes {
		if resourceSlice.Spec.Class == class {
			return true
		}
	}
	return false
}

func acceptResources(resourceSlice *authv1alpha1.ResourceSlice, er record.EventRecorder) {
	switch authentication.EnsureCondition(
		resourceSlice,
		authv1alpha1.ResourceSliceConditionTypeResources,
		authv1alpha1.ResourceSliceConditionAccepted,
		"ResourceSliceResourcesAccepted",
		"ResourceSlice resources accepted",
	) {
	case controllerutil.OperationResultNone:
		klog.V(4).Infof("ResourceSlice resources %q already accepted", resourceSlice.Name)
	case controllerutil.OperationResultUpdated:
		klog.Infof("Resources %q updated", resourceSlice.Name)
		er.Event(resourceSlice, corev1.EventTypeNormal, "ResourceSliceResourcesAccepted", "ResourceSlice resources updated")
	case controllerutil.OperationResultCreated:
		klog.Infof("Resources %q accepted", resourceSlice.Name)
		er.Event(resourceSlice, corev1.EventTypeNormal, "ResourceSliceResourcesAccepted", "ResourceSlice resources accepted")
	default:
		return
	}
}

func withCSR() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		rs, ok := obj.(*authv1alpha1.ResourceSlice)
		if !ok {
			return false
		}
		return rs.Spec.CSR != nil && len(rs.Spec.CSR) > 0
	})
}
