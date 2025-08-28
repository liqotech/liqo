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

package shadowendpointslicectrl

import (
	"context"
	"fmt"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
	clientutils "github.com/liqotech/liqo/pkg/utils/clients"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	"github.com/liqotech/liqo/pkg/utils/resource"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

const ctrlFieldManager = "shadow-endpointslice-controller"

// Reconciler reconciles a ShadowEndpointSlice object.
type Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	DenyDirectConnections bool
}

// +kubebuilder:rbac:groups=offloading.liqo.io,resources=shadowendpointslices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.liqo.io,resources=foreignclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.liqo.io,resources=foreignclusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=connections,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch

// Reconcile ShadowEndpointSlices objects.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	nsName := req.NamespacedName
	klog.V(4).Infof("reconcile shadowendpointslice %q", nsName)

	var shadowEps offloadingv1beta1.ShadowEndpointSlice
	if err := r.Get(ctx, nsName, &shadowEps); err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("shadowendpointslice %q not found", nsName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("an error occurred while getting shadowendpointslice %q: %v", nsName, err)
		return ctrl.Result{}, err
	}

	// Get ForeignCluster associated with the shadowendpointslice
	clusterID, ok := utils.GetClusterIDFromLabelsWithKey(shadowEps.Labels, forge.LiqoOriginClusterIDKey)
	if !ok {
		klog.Errorf("shadowendpointslice %q has no label %q", nsName, forge.LiqoOriginClusterIDKey)
		return ctrl.Result{}, fmt.Errorf("shadowendpointslice %q has no label %q", nsName, forge.LiqoOriginClusterIDKey)
	}
	fc, err := foreigncluster.GetForeignClusterByID(ctx, r.Client, clusterID)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Errorf("foreigncluster (id: %q) associated with shadowendpointslice %q not found", clusterID, nsName)
			return ctrl.Result{}, err
		}
		klog.Errorf("an error occurred while getting foreigncluster %q: %v", clusterID, err)
		return ctrl.Result{}, err
	}

	// Check network status of the foreigncluster
	networkReady := foreigncluster.IsNetworkingEstablishedOrDisabled(fc)

	// Check foreign API server status
	apiServerReady := foreigncluster.IsAPIServerReadyOrDisabled(fc)

	// Classify the slice with respect to the direct-connections feature and check the usability
	// of the direct path of the endpoints in this shadoweps (see directconnections.go).
	dp, err := r.resolveDirectPath(ctx, &shadowEps)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("shadowendpointslice %q: %w", nsName, err)
	}

	// Get the endpoints from the shadowendpointslice and remap them if necessary.
	remappedEndpoints := shadowEps.Spec.Template.Endpoints

	// Index of the addresses reachable through direct connections, and per-endpoint
	// classification (the direct cluster each endpoint depends on, "" if none). Classification
	// must happen before the remapping below rewrites the addresses.
	translationIndex := dp.data.BuildIndex()
	var endpointClusters []string
	if dp.isDirect() {
		endpointClusters = classifyEndpoints(remappedEndpoints, translationIndex)
	}

	// Never-peered misconfiguration
	if dp.isDirect() && dp.state == directPathNotPeered {
		r.reportNotPeered(ctx, &shadowEps, dp.notPeered)
		remappedEndpoints, endpointClusters = dropEndpointsOfClusters(remappedEndpoints, endpointClusters, dp.notPeered.Clusters)
	}

	if dp.isDirect() && dp.state == directPathDenied {
		// Direct EndpointSlices can lack a matching Configuration when direct connections
		// are denied, so disable translation to avoid remapping errors in this corner case.
		translationIndex = nil
	}

	if foreigncluster.IsNetworkingModuleEnabled(fc) {
		// remap the endpoints if the network configuration of the remote cluster overlaps with the local one
		if err := MapEndpointsWithConfiguration(ctx, r.Client, clusterID, remappedEndpoints, translationIndex); err != nil {
			return ctrl.Result{}, fmt.Errorf("an error occurred while remapping endpoints for shadowendpointslice %q: %w", nsName, err)
		}
	} else if translationIndex != nil {
		if err := MapOnlyDirectConnectionEndpoints(ctx, r.Client, remappedEndpoints, translationIndex); err != nil {
			return ctrl.Result{}, fmt.Errorf("an error occurred while remapping direct-connection endpoints for shadowendpointslice %q: %w", nsName, err)
		}
	}

	// Direct connections data annotation is not propagated to the EndpointSlice
	annotations := removeDirectConnectionAnnotation(shadowEps.GetAnnotations())

	// Forge the endpointslice given the shadowendpointslice
	newEps := discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shadowEps.Name,
			Namespace: shadowEps.Namespace,
			Labels: labels.Merge(shadowEps.Labels, labels.Set{
				consts.ManagedByLabelKey: consts.ManagedByShadowEndpointSliceValue}),
			Annotations: annotations,
		},
		AddressType: shadowEps.Spec.Template.AddressType,
		Endpoints:   remappedEndpoints,
		Ports:       shadowEps.Spec.Template.Ports,
	}

	applyEndpointsReadiness(newEps.Endpoints, endpointClusters, &dp, networkReady, apiServerReady)

	// Get existing endpointslice if it is already been created from the shadowendpointslice
	var existingEps discoveryv1.EndpointSlice
	err = r.Get(ctx, nsName, &existingEps)

	switch {
	case errors.IsNotFound(err):
		// Create the endpointslice
		utilruntime.Must(ctrl.SetControllerReference(&shadowEps, &newEps, r.Scheme))

		resource.AddGlobalLabels(&newEps)
		resource.AddGlobalAnnotations(&newEps)

		if err := r.Create(ctx, &newEps, client.FieldOwner(ctrlFieldManager)); err != nil {
			klog.Errorf("unable to create endpointslice for shadowendpointslice %q: %v", klog.KObj(&shadowEps), err)
			return ctrl.Result{}, err
		}

		klog.Infof("created endpointslice %q for shadowendpointslice %q", klog.KObj(&newEps), klog.KObj(&shadowEps))

		return ctrl.Result{}, nil

	case err != nil:
		klog.Errorf("unable to get endpointslice %q: %v", nsName, err)
		return ctrl.Result{}, err

	default:
		klog.V(4).Infof("endpointslice %q found running, will update it", klog.KObj(&existingEps))

		// Create Apply object for existing endpointslice
		epsApply := EndpointSliceApply(&newEps)

		if err := r.Patch(ctx, &existingEps, clientutils.Patch(epsApply),
			client.ForceOwnership, client.FieldOwner(ctrlFieldManager)); err != nil {
			klog.Errorf("unable to update endpointslice %q: %v", klog.KObj(&existingEps), err)
			return ctrl.Result{}, err
		}

		klog.Infof("updated endpointslice %q with success", klog.KObj(&existingEps))

		return ctrl.Result{}, nil
	}
}

// getForeignClusterEventHandler returns an event handler that reacts on ForeignClusters updates.
// In particular, it reacts on changes on the NetworkStatus condition.
func (r *Reconciler) getForeignClusterEventHandler(ctx context.Context) handler.EventHandler {
	return &handler.Funcs{
		UpdateFunc: func(_ context.Context, ue event.TypedUpdateEvent[client.Object], trli workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			newForeignCluster, ok := ue.ObjectNew.(*liqov1beta1.ForeignCluster)
			if !ok {
				klog.Errorf("object %v is not a ForeignCluster", ue.ObjectNew)
				return
			}

			clusterID := newForeignCluster.Spec.ClusterID
			if clusterID == "" {
				klog.Errorf("cluster-id not set on foreignCluster %v", newForeignCluster)
				return
			}

			// List all shadowendpointslices with clusterID as origin
			var shadowList offloadingv1beta1.ShadowEndpointSliceList
			if err := r.List(ctx, &shadowList, client.MatchingLabels{forge.LiqoOriginClusterIDKey: string(clusterID)}); err != nil {
				klog.Errorf("Unable to list shadowendpointslices")
				return
			}

			for i := range shadowList.Items {
				shadow := &shadowList.Items[i]
				trli.Add(reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      shadow.Name,
						Namespace: shadow.Namespace,
					},
				})
			}
		},
	}
}

func (r *Reconciler) endpointsShouldBeUpdated(newObj, oldObj client.Object) bool {
	oldForeignCluster, ok := oldObj.(*liqov1beta1.ForeignCluster)
	if !ok {
		klog.Errorf("object %v is not a ForeignCluster", oldObj)
		return false
	}

	newForeignCluster, ok := newObj.(*liqov1beta1.ForeignCluster)
	if !ok {
		klog.Errorf("object %v is not a ForeignCluster", newObj)
		return false
	}

	oldFcNetworkReady := foreigncluster.IsNetworkingEstablishedOrDisabled(oldForeignCluster)
	newFcNetworkReady := foreigncluster.IsNetworkingEstablishedOrDisabled(newForeignCluster)

	oldFcAPIServerReady := foreigncluster.IsAPIServerReadyOrDisabled(oldForeignCluster)
	newFcAPIServerReady := foreigncluster.IsAPIServerReadyOrDisabled(newForeignCluster)

	// Reconcile if the network status or the API server status changed
	return oldFcNetworkReady != newFcNetworkReady || oldFcAPIServerReady != newFcAPIServerReady
}

// SetupWithManager monitors updates on ShadowEndpointSlices.
func (r *Reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, workers int) error {
	// Trigger a reconciliation only for Update Events on NetworkStatus of the ForeignCluster.
	fcPredicates := predicate.Funcs{
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		CreateFunc:  func(_ event.CreateEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return r.endpointsShouldBeUpdated(e.ObjectNew, e.ObjectOld) },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlShadowEndpointSlice).
		For(&offloadingv1beta1.ShadowEndpointSlice{}).
		Owns(&discoveryv1.EndpointSlice{}).
		Watches(&liqov1beta1.ForeignCluster{},
			r.getForeignClusterEventHandler(ctx), builder.WithPredicates(fcPredicates)).
		// Direct-connections failover: re-enqueue the involved slices when the status of a
		// Connection towards another provider changes (see connection_watches.go).
		Watches(&networkingv1beta1.Connection{},
			r.getConnectionEventHandler(),
			builder.WithPredicates(connectionStatusChangedPredicate())).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}
