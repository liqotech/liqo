// Copyright 2019-2026 The Liqo Authors
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

package virtualnodectrl

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/indexer"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

const (
	// virtualNodeControllerFinalizer is the finalizer added to virtual-node to allow the controller to clean up.
	virtualNodeControllerFinalizer = "virtualnode-controller.liqo.io/finalizer"
	// vkOptionsTemplateRefIndexField is the field used to index the VirtualNodes by the referenced VkOptionsTemplate.
	vkOptionsTemplateRefIndexField = "spec.vkOptionsTemplateRef"
)

// VirtualNodeReconciler manage NamespaceMap lifecycle.
type VirtualNodeReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder

	HomeClusterID         liqov1beta1.ClusterID
	LiqoNamespace         string
	LocalPodCIDRs         []string
	VkOptsDefaultTemplate *corev1.ObjectReference

	namespaceManager tenantnamespace.Manager
	dr               *DeletionRoutine
}

// VirtualNodeReconcilerOptions contains the options to create a new VirtualNodeReconciler.
type VirtualNodeReconcilerOptions struct {
	HomeClusterID         liqov1beta1.ClusterID
	LiqoNamespace         string
	LocalPodCIDRs         []string
	VkOptsDefaultTemplate *corev1.ObjectReference
}

// NewVirtualNodeReconciler returns a new VirtualNodeReconciler.
func NewVirtualNodeReconciler(
	ctx context.Context,
	cl client.Client,
	s *runtime.Scheme,
	er record.EventRecorder,
	namespaceManager tenantnamespace.Manager,
	opts VirtualNodeReconcilerOptions,
) (*VirtualNodeReconciler, error) {
	vnr := &VirtualNodeReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,

		HomeClusterID:         opts.HomeClusterID,
		LiqoNamespace:         opts.LiqoNamespace,
		LocalPodCIDRs:         opts.LocalPodCIDRs,
		VkOptsDefaultTemplate: opts.VkOptsDefaultTemplate,

		namespaceManager: namespaceManager,
	}
	var err error
	vnr.dr, err = RunDeletionRoutine(ctx, vnr)
	if err != nil {
		return nil, fmt.Errorf("running virtualnode deletion routine: %w", err)
	}
	return vnr, nil
}

// cluster-role
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=virtualnodes,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=virtualnodes/status,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=virtualnodes/finalizers,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespacemaps,verbs=get;list;watch;delete;create
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=vkoptionstemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

// Reconcile manage NamespaceMaps associated with the virtual-node.
func (r *VirtualNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	virtualNode := &offloadingv1beta1.VirtualNode{}
	if err := r.Get(ctx, req.NamespacedName, virtualNode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no virtual-node called %q in %q", req.Name, req.Namespace)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the virtual-node %q: %w", req.NamespacedName, err)
	}

	if virtualNode.DeletionTimestamp.IsZero() {
		if !ctrlutil.ContainsFinalizer(virtualNode, virtualNodeControllerFinalizer) {
			if err := r.ensureVirtualNodeFinalizerPresence(ctx, virtualNode); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if ctrlutil.ContainsFinalizer(virtualNode, virtualNodeControllerFinalizer) {
			// If the virtual-node is being deleted, it deletes the node and the virtual-node resource.
			if err := r.dr.EnsureNodeAbsence(virtualNode); err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to delete the virtual-node: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	// The embedded deployment template is deprecated: the deployment is now deterministically
	// forged from the VkOptionsTemplate at every reconcile. Remove the legacy field if set.
	if virtualNode.Spec.Template != nil { //nolint:staticcheck // reading the deprecated field to clear it
		if err := r.clearLegacyTemplate(ctx, virtualNode); err != nil {
			return ctrl.Result{}, fmt.Errorf("clearing legacy template: %w", err)
		}
	}

	// Resolve the VkOptionsTemplate referenced by the VirtualNode, defaulting to the configured one.
	vkOpts, err := r.getVkOptionsTemplate(ctx, virtualNode)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("resolving the VkOptionsTemplate: %w", err)
	}

	// If the VirtualNode is annotated to skip the virtual-kubelet deployment, only the
	// meta-management (finalizer, NamespaceMap) is performed: the deployment (and its
	// supporting resources) is expected to be managed externally.
	skipVkDeployment := virtualNode.Annotations != nil &&
		virtualNode.Annotations[consts.SkipVkDeploymentAnnotation] != "" &&
		!strings.EqualFold(virtualNode.Annotations[consts.SkipVkDeploymentAnnotation], "false")
	if skipVkDeployment {
		klog.V(4).Infof("Skipping the virtual-kubelet deployment for the annotated virtual-node %q", req.NamespacedName)
	} else {
		if err := r.ensureVirtualKubeletDeploymentPresence(ctx, virtualNode, vkOpts); err != nil {
			return ctrl.Result{}, fmt.Errorf("ensuring virtual-kubelet deployment presence: %w", err)
		}
		if !vkforge.EffectiveCreateNode(virtualNode, vkOpts) {
			// If the virtual-node is not enabled, it deletes the node but not the virtual-node resource.
			if err := r.dr.EnsureNodeAbsence(virtualNode); err != nil {
				return ctrl.Result{}, fmt.Errorf("deleting the node: %w", err)
			}
		}
	}

	// If there is no NamespaceMap associated with this virtual-node, it creates a new one.
	if err := r.ensureNamespaceMapPresence(ctx, virtualNode); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *VirtualNodeReconciler) enqueFromNamespaceMap() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, o client.Object) []reconcile.Request {
			nm, ok := o.(*offloadingv1beta1.NamespaceMap)
			if !ok {
				return []reconcile.Request{}
			}

			if nm.Labels == nil || nm.Labels[consts.RemoteClusterID] == "" {
				return []reconcile.Request{}
			}
			clusterID := nm.Labels[consts.RemoteClusterID]

			// list virtualnode resources with the remote cluster ID label
			virtualnodes, err := getters.ListVirtualNodesByClusterID(ctx, r.Client, liqov1beta1.ClusterID(clusterID))
			if err != nil {
				klog.Errorf("unable to list virtualnodes with clusterID %s: %v", clusterID, err)
				return []reconcile.Request{}
			}

			requests := []reconcile.Request{}
			for i := range virtualnodes {
				requests = append(requests, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(&virtualnodes[i]),
				})
			}
			return requests
		})
}

// clearLegacyTemplate removes the deprecated embedded deployment template from the given VirtualNode.
func (r *VirtualNodeReconciler) clearLegacyTemplate(ctx context.Context, virtualNode *offloadingv1beta1.VirtualNode) error {
	virtualNode.Spec.Template = nil //nolint:staticcheck // clearing the deprecated field
	if err := r.Update(ctx, virtualNode); err != nil {
		return fmt.Errorf("updating virtual node %q: %w", client.ObjectKeyFromObject(virtualNode), err)
	}

	klog.Infof("Removed the legacy deployment template from virtual-node %q", client.ObjectKeyFromObject(virtualNode))
	r.EventsRecorder.Event(virtualNode, "Normal", "LegacyTemplateRemoved", "The legacy deployment template has been removed")

	return nil
}

// vkOptionsTemplateRefIndexer returns the indexer function that maps each VirtualNode to
// the key of the VkOptionsTemplate it references, falling back to the configured default
// one when no explicit reference is set.
func (r *VirtualNodeReconciler) vkOptionsTemplateRefIndexer() client.IndexerFunc {
	return func(rawObj client.Object) []string {
		virtualNode, ok := rawObj.(*offloadingv1beta1.VirtualNode)
		if !ok {
			return nil
		}
		ref := virtualNode.Spec.VkOptionsTemplateRef
		if ref == nil {
			ref = r.VkOptsDefaultTemplate
		}
		if ref == nil {
			return nil
		}
		return []string{types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}.String()}
	}
}

// enqueueFromVkOptionsTemplate enqueues the VirtualNodes affected by a change of the given VkOptionsTemplate.
func (r *VirtualNodeReconciler) enqueueFromVkOptionsTemplate() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, o client.Object) []reconcile.Request {
			vkOpts, ok := o.(*offloadingv1beta1.VkOptionsTemplate)
			if !ok {
				return []reconcile.Request{}
			}

			key := client.ObjectKeyFromObject(vkOpts).String()
			virtualnodes := &offloadingv1beta1.VirtualNodeList{}
			if err := r.List(ctx, virtualnodes,
				client.MatchingFields{vkOptionsTemplateRefIndexField: key}); err != nil {
				klog.Errorf("listing virtualnodes referencing the VkOptionsTemplate %q: %v", key, err)
				return []reconcile.Request{}
			}

			requests := make([]reconcile.Request, 0, len(virtualnodes.Items))
			for i := range virtualnodes.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(&virtualnodes.Items[i]),
				})
			}
			return requests
		})
}

// SetupWithManager register the VirtualNodeReconciler to the manager.
func (r *VirtualNodeReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// Index the VirtualNodes by the referenced VkOptionsTemplate, to react to template changes.
	if err := indexer.IndexField(ctx, mgr, &offloadingv1beta1.VirtualNode{},
		vkOptionsTemplateRefIndexField, r.vkOptionsTemplateRefIndexer()); err != nil {
		return fmt.Errorf("setting up the VkOptionsTemplate reference indexer: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlVirtualNode).
		For(&offloadingv1beta1.VirtualNode{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ServiceAccount{}).
		Watches(&offloadingv1beta1.VkOptionsTemplate{}, r.enqueueFromVkOptionsTemplate()).
		Watches(&offloadingv1beta1.NamespaceMap{}, r.enqueFromNamespaceMap()).
		Complete(r)
}
