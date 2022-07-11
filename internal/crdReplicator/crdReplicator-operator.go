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

package crdreplicator

import (
	"context"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/internal/crdReplicator/resources"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
)

const (
	operatorName = "crdReplicator-operator"
	finalizer    = "crdReplicator.liqo.io"
)

// Controller reconciles ForeignCluster objects to start/stop the reflection of registered resources to remote clusters.
type Controller struct {
	Scheme *runtime.Scheme
	client.Client

	// Local cluster ID
	ClusterID string
	// Local cluster name
	ClusterName string

	// RegisteredResources is a list of GVRs of resources to be replicated, with the associated peering phase when the replication has to occur.
	RegisteredResources []resources.Resource

	// ReflectionManager is the object managing the reflection towards remote clusters.
	ReflectionManager *reflection.Manager

	// ExternalReflectors is a map containing the reflectors towards each remote cluster.
	// Each reflector reads on the local tenant namespace and writes on the remote tenant namespace.
	ExternalReflectors map[string]*reflection.Reflector
	// InternalReflector is a reflector between namespaces in the same local cluster.
	// It reads and writes on pairs of local ns.
	InternalReflector *reflection.Reflector

	// IdentityReader is an interface to manage remote identities, and to get the rest config.
	IdentityReader identitymanager.IdentityReader

	// peeringPhases maps a remote cluster ID to its peering phase
	peeringPhases      map[string]consts.PeeringPhase
	peeringPhasesMutex sync.RWMutex

	networkingStates     map[string]discoveryv1alpha1.NetworkingEnabledType
	networkingStateMutex sync.RWMutex
}

// cluster-role
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get
// role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=configmaps,verbs=get;list

// identity management
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list

// Reconcile handles requests for subscribed types of object.
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	tracer := trace.New("Reconcile", trace.Field{Key: "ForeignCluster", Value: req.Name})
	defer tracer.LogIfLong(traceutils.LongThreshold())

	var fc discoveryv1alpha1.ForeignCluster
	err = c.Get(ctx, req.NamespacedName, &fc)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("Unable to retrieve resource %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}

	remoteCluster := fc.Spec.ClusterIdentity
	klog.Infof("[%v] Processing ForeignCluster %q", remoteCluster.ClusterName, fc.Name)
	// Prevent issues in case the remote cluster ID has not yet been set
	if remoteCluster.ClusterID == "" {
		klog.Infof("Remote Cluster ID is not yet set in resource %q", fc.Name)
		return ctrl.Result{}, nil
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if !fc.ObjectMeta.DeletionTimestamp.IsZero() {
		// the object is being deleted
		if controllerutil.ContainsFinalizer(&fc, finalizer) {
			// close remote watcher for remote cluster
			reflector, ok := c.ExternalReflectors[remoteCluster.ClusterID]
			if ok {
				if err := reflector.Stop(c.ClusterID, remoteCluster.ClusterID); err != nil {
					klog.Errorf("[%v] Failed to stop reflection: %v", remoteCluster.ClusterName, err)
					return ctrl.Result{}, err
				}
				delete(c.ExternalReflectors, remoteCluster.ClusterID)
			}

			// remove the finalizer from the list and update it.
			if err := c.ensureFinalizer(ctx, &fc, false, controllerutil.RemoveFinalizer); err != nil {
				klog.Errorf("An error occurred while removing the finalizer to %q: %s", fc.Name, err)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		// CHECK return?
	}

	// Defer the function to start/stop the reflection of the different resources based on the peering status.
	defer func() {
		if err == nil {
			err = c.enforceReflectionStatus(ctx, remoteCluster, !fc.ObjectMeta.DeletionTimestamp.IsZero())
		}
	}()

	currentPhase := foreigncluster.GetPeeringPhase(&fc)
	// The remote identity is not yet available, hence it is not possible to continue.
	if currentPhase == consts.PeeringPhaseNone {
		klog.Infof("Foreign cluster %s not yet associated with a valid identity", fc.Name)
		return ctrl.Result{}, nil
	}

	if currentPhase == consts.PeeringPhaseInduced {
		// No need to proceed, since the reconciled foreign cluster is related to an induced peering
		return ctrl.Result{}, nil
	}

	// Add the finalizer to ensure the reflection is correctly stopped
	if err := c.ensureFinalizer(ctx, &fc, true, controllerutil.AddFinalizer); err != nil {
		klog.Errorf("An error occurred while adding the finalizer to %q: %s", fc.Name, err)
		return ctrl.Result{}, err
	}

	// The first time, oldPhase will be "None" since the "c.peeringPhases" map is empty.
	// "currentPhase" will be different from "None" and therefore "c.setPeeringPhase" will be called.
	if oldPhase := c.getPeeringPhase(remoteCluster.ClusterID); oldPhase != currentPhase {
		klog.V(4).Infof("[%v] Peering phase changed: old: %v, new: %v", remoteCluster.ClusterName, oldPhase, currentPhase)
		c.setPeeringPhase(remoteCluster.ClusterID, currentPhase)
	}

	currentNetState := foreigncluster.GetNetworkingState(&fc)
	if oldNetState := c.getNetworkingState(remoteCluster.ClusterID); oldNetState != currentNetState {
		klog.V(4).Infof("[%v] Networking state changed: old: %v, new %v", remoteCluster.ClusterName, oldNetState, currentNetState)
		c.setNetworkingState(remoteCluster.ClusterID, currentNetState)
	}

	// Check if reflection towards the remote cluster has already been started.
	if _, found := c.ExternalReflectors[remoteCluster.ClusterID]; found {
		return ctrl.Result{}, nil
	}

	if fc.Status.TenantNamespace.Local == "" || fc.Status.TenantNamespace.Remote == "" {
		klog.Infof("[%v] TenantNamespace is not yet set in resource %q", remoteCluster.ClusterName, fc.Name)
		return ctrl.Result{}, nil
	}

	var clusterIdentity discoveryv1alpha1.ClusterIdentity
	if fc.Spec.InducedPeering.InducedPeeringEnabled == discoveryv1alpha1.PeeringEnabledYes {
		clusterIdentity = fc.Spec.InducedPeering.OriginClusterIdentity
	} else {
		clusterIdentity = remoteCluster
	}
	config, err := c.IdentityReader.GetConfig(clusterIdentity, fc.Status.TenantNamespace.Local)
	if err != nil {
		klog.Errorf("[%v] Unable to retrieve config from resource %q: %s", remoteCluster.ClusterName, fc.Name, err)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, c.setupReflection(ctx, config, &fc)
}

// SetupWithManager registers a new controller for ForeignCluster resources.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	resourceToBeProcessedPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).Named(operatorName).WithEventFilter(resourceToBeProcessedPredicate).
		For(&discoveryv1alpha1.ForeignCluster{}).
		Complete(c)
}

// ensureFinalizer updates the ForeignCluster to ensure the presence/absence of the finalizer.
func (c *Controller) ensureFinalizer(ctx context.Context, foreignCluster *discoveryv1alpha1.ForeignCluster,
	expected bool, updater func(client.Object, string)) error {
	// Do not perform any action if the finalizer is already present (i.e. expected is true) or absent (expected is false)
	if controllerutil.ContainsFinalizer(foreignCluster, finalizer) == expected {
		return nil
	}

	updater(foreignCluster, finalizer)
	return c.Client.Update(ctx, foreignCluster)
}

func (c *Controller) setupReflection(ctx context.Context, config *rest.Config, fc *discoveryv1alpha1.ForeignCluster) error {
	if fc.Spec.InducedPeering.InducedPeeringEnabled != discoveryv1alpha1.PeeringEnabledYes {
		if err := c.setupExternalReflection(ctx, config, fc); err != nil {
			return err
		}

		// The following is done only when reconciling full-peering foreing clusters,
		// in order to configure the correct local-to-local namespaces.
		// The reason is that when an induced-peering foreing cluster is reconciled, for sure
		// the local cluster is a leaf cluster and therefore the following does not have to be done
		// using the data of an induced-peering foreign cluster.
		c.setupInternalReflection(fc)
	}

	return nil
}

func (c *Controller) setupExternalReflection(ctx context.Context, config *rest.Config, fc *discoveryv1alpha1.ForeignCluster) error {
	remoteClusterID := fc.Spec.ClusterIdentity.ClusterID
	localNamespace := fc.Status.TenantNamespace.Local
	remoteNamespace := fc.Status.TenantNamespace.Remote

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Errorf("[%v] Unable to create dynamic client for remote cluster: %v", remoteClusterID, err)
		return err
	}

	externalReflector := c.ReflectionManager.NewForTarget(dynamicClient, remoteClusterID, localNamespace, remoteNamespace, false)
	externalReflector.Start(ctx)
	c.ExternalReflectors[remoteClusterID] = externalReflector

	return nil
}

func (c *Controller) setupInternalReflection(fc *discoveryv1alpha1.ForeignCluster) {
	remoteClusterID := fc.Spec.ClusterIdentity.ClusterID
	remoteClusterName := fc.Spec.ClusterIdentity.ClusterName
	localNamespace := fc.Status.TenantNamespace.Local
	c.InternalReflector.TenantNamespaces[remoteClusterID] = localNamespace
	c.InternalReflector.ClusterNames[remoteClusterID] = remoteClusterName
}

func (c *Controller) enforceReflectionStatus(ctx context.Context, remoteCluster discoveryv1alpha1.ClusterIdentity, deleting bool) error {
	remoteClusterID := remoteCluster.ClusterID
	remoteClusterName := remoteCluster.ClusterName
	reflector, found := c.ExternalReflectors[remoteClusterID]
	if !found {
		// The reflector object has not yet been setup
		return nil
	}

	netState := c.getNetworkingState(remoteClusterID)
	phase := c.getPeeringPhase(remoteClusterID)
	for i := range c.RegisteredResources {
		res := &c.RegisteredResources[i]
		if reflector != nil {
			if !deleting && isReplicationAllowed(phase, res) && isNetworkingEnabled(netState, res) && !reflector.ResourceStarted(res, c.ClusterID, remoteClusterID) {
				reflector.StartForResource(ctx, res, c.ClusterID, remoteClusterID, c.ClusterID, c.ClusterName, remoteClusterName)
			} else if (!isReplicationAllowed(phase, res) || !isNetworkingEnabled(netState, res)) && reflector.ResourceStarted(res, c.ClusterID, remoteClusterID) {
				if err := reflector.StopForResource(res, c.ClusterID, remoteClusterID); err != nil {
					return err
				}
			}
		}
		//TODO Check network enabled for induced
		if res.Forwardable && len(c.InternalReflector.TenantNamespaces) >= 2 {
			for k1 := range c.InternalReflector.TenantNamespaces {
				for k2 := range c.InternalReflector.TenantNamespaces {
					// Skip if k1 and k2 refer to the same clusterID
					if k1 == k2 {
						continue
					}
					if !deleting && !c.InternalReflector.ResourceStarted(res, k1, k2) {
						c.InternalReflector.StartForResource(ctx, res, k1, k2, c.ClusterID, c.InternalReflector.ClusterNames[k1], c.InternalReflector.ClusterNames[k2])
					} else if deleting && c.InternalReflector.ResourceStarted(res, k1, k2) {
						if err := c.InternalReflector.StopForResource(res, k1, k2); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}
