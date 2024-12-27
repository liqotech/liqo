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

package virtualnodecreatorcontroller

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/forge"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// VirtualNodeCreatorReconciler create virtualnodes from resourceslice resources.
type VirtualNodeCreatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	EventRecorder record.EventRecorder
}

// NewVirtualNodeCreatorReconciler returns a new VirtualNodeCreatorReconciler.
func NewVirtualNodeCreatorReconciler(cl client.Client, s *runtime.Scheme, recorder record.EventRecorder) *VirtualNodeCreatorReconciler {
	return &VirtualNodeCreatorReconciler{
		Client: cl,
		Scheme: s,

		EventRecorder: recorder,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=virtualnodes,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=resourceslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=identities,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;delete;create;update;patch

// Reconcile resourceslices and create their associated virtualnodes.
func (r *VirtualNodeCreatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var resourceSlice authv1beta1.ResourceSlice
	if err := r.Get(ctx, req.NamespacedName, &resourceSlice); err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("resourceSlice %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("unable to get ResourceSlice %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if resourceSlice.DeletionTimestamp != nil {
		klog.V(6).Infof("ResourceSlice %q is being deleted", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	if resourceSlice.Annotations == nil ||
		resourceSlice.Annotations[consts.CreateVirtualNodeAnnotation] == "" ||
		strings.EqualFold(resourceSlice.Annotations[consts.CreateVirtualNodeAnnotation], "false") {
		klog.V(6).Infof("VirtualNode creation disabled for resourceslice %q", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	if !allConditionsAccepted(&resourceSlice) {
		klog.V(6).Infof("Not all ResourceSlice %q conditions are yet accepted", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	if resourceSlice.Labels == nil || resourceSlice.Labels[consts.RemoteClusterID] == "" {
		err := fmt.Errorf("resourceslice %q does not contain the remote cluster ID label", req.NamespacedName)
		klog.Error(err)
		return ctrl.Result{}, err
	}

	remoteClusterID := liqov1beta1.ClusterID(resourceSlice.Labels[consts.RemoteClusterID])

	// Get the associated Identity for the remote cluster.
	identity, err := getters.GetIdentityFromResourceSlice(ctx, r.Client, remoteClusterID, resourceSlice.Name)
	if err != nil {
		klog.Errorf("Unable to get the Identity associated to resourceslice %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}
	if identity.Status.KubeconfigSecretRef == nil || identity.Status.KubeconfigSecretRef.Name == "" {
		klog.V(6).Infof("Identity %q does not contain the kubeconfig secret reference yet", identity.Name)
		return ctrl.Result{}, nil
	}

	// Get associated secret
	kubeconfigSecret, err := getters.GetKubeconfigSecretFromIdentity(ctx, r.Client, identity)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to get the kubeconfig secret from identity %q: %w", identity.Name, err)
	}

	// CreateOrUpdate the VirtualNode.
	virtualNode := forge.VirtualNode(resourceSlice.Name, resourceSlice.Namespace)
	if _, err := resource.CreateOrUpdate(ctx, r.Client, virtualNode, func() error {
		// Forge the VirtualNodeOptions from the ResourceSlice.
		vnOpts := forge.VirtualNodeOptionsFromResourceSlice(&resourceSlice, kubeconfigSecret.Name,
			virtualNode.Spec.VkOptionsTemplateRef)

		if err := forge.MutateVirtualNode(ctx, r.Client,
			virtualNode, identity.Spec.ClusterID, vnOpts, nil, nil, nil); err != nil {
			return err
		}
		if virtualNode.Labels == nil {
			virtualNode.Labels = map[string]string{}
		}
		virtualNode.Labels[consts.ResourceSliceNameLabelKey] = resourceSlice.Name
		return ctrl.SetControllerReference(&resourceSlice, virtualNode, r.Scheme)
	}); err != nil {
		klog.Errorf("Unable to create or update the VirtualNode for resourceslice %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	klog.Infof("VirtualNode created for resourceslice %q and cluster %q", req.NamespacedName, remoteClusterID)
	r.EventRecorder.Event(&resourceSlice, "Normal", "VirtualNodeCreated", "VirtualNode created for resourceslice")
	return ctrl.Result{}, nil
}

// SetupWithManager register the VirtualNodeCreatorReconciler with the manager.
func (r *VirtualNodeCreatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// generate the predicate to filter just the ResourceSlices created by the local cluster checking crdReplicator labels
	localResSliceFilter, err := predicate.LabelSelectorPredicate(reflection.LocalResourcesLabelSelector())
	if err != nil {
		klog.Error(err)
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlResourceSliceVNCreator).
		For(&authv1beta1.ResourceSlice{}, builder.WithPredicates(predicate.And(localResSliceFilter, withConditionsAccepeted()))).
		Owns(&offloadingv1beta1.VirtualNode{}).
		Owns(&authv1beta1.Identity{}).
		Complete(r)
}

func withConditionsAccepeted() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		rs, ok := obj.(*authv1beta1.ResourceSlice)
		if !ok {
			return false
		}

		return allConditionsAccepted(rs)
	})
}

func allConditionsAccepted(rs *authv1beta1.ResourceSlice) bool {
	authCond := authentication.GetCondition(rs, authv1beta1.ResourceSliceConditionTypeAuthentication)
	authAccepted := authCond != nil && authCond.Status == authv1beta1.ResourceSliceConditionAccepted

	resourcesCond := authentication.GetCondition(rs, authv1beta1.ResourceSliceConditionTypeResources)
	resourcesAccepted := resourcesCond != nil && resourcesCond.Status == authv1beta1.ResourceSliceConditionAccepted

	return authAccepted && resourcesAccepted
}
