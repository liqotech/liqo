// Copyright 2019-2023 The Liqo Authors
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
	ClusterID string

	// RegisteredResources is a list of GVRs of resources to be replicated, with the associated peering phase when the replication has to occur.
	RegisteredResources []resources.Resource

	// ReflectionManager is the object managing the reflection towards remote clusters.
	ReflectionManager *reflection.Manager
	// Reflectors is a map containing the reflectors towards each remote cluster.
	Reflectors map[string]*reflection.Reflector

	// IdentityReader is an interface to manage remote identities, and to get the rest config.
	IdentityReader identitymanager.IdentityReader

	peeringPhases      map[string]consts.PeeringPhase
	peeringPhasesMutex sync.RWMutex

	networkingEnabled      map[string]bool
	networkingEnabledMutex sync.RWMutex
}

// cluster-role
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get
// role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=configmaps,verbs=get;list;watch

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
			reflector, ok := c.Reflectors[remoteCluster.ClusterID]
			if ok {
				if err := reflector.Stop(); err != nil {
					klog.Errorf("[%v] Failed to stop reflection: %v", remoteCluster.ClusterName, err)
					return ctrl.Result{}, err
				}
				delete(c.Reflectors, remoteCluster.ClusterID)
			}

			// remove the finalizer from the list and update it.
			if err := c.ensureFinalizer(ctx, &fc, controllerutil.RemoveFinalizer); err != nil {
				klog.Errorf("An error occurred while removing the finalizer to %q: %s", fc.Name, err)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	// Defer the function to start/stop the reflection of the different resources based on the peering status.
	defer func() {
		if err == nil {
			err = c.enforceReflectionStatus(ctx, remoteCluster.ClusterID, !fc.ObjectMeta.DeletionTimestamp.IsZero())
		}
	}()

	currentPhase := foreigncluster.GetPeeringPhase(&fc)
	// The remote identity is not yet available, hence it is not possible to continue.
	if currentPhase == consts.PeeringPhaseNone {
		klog.Infof("Foreign cluster %s not yet associated with a valid identity", fc.Name)
		return ctrl.Result{}, nil
	}

	// Add the finalizer to ensure the reflection is correctly stopped
	if err := c.ensureFinalizer(ctx, &fc, controllerutil.AddFinalizer); err != nil {
		klog.Errorf("An error occurred while adding the finalizer to %q: %s", fc.Name, err)
		return ctrl.Result{}, err
	}

	if oldPhase := c.getPeeringPhase(remoteCluster.ClusterID); oldPhase != currentPhase {
		klog.V(4).Infof("[%v] Peering phase changed: old: %v, new: %v", remoteCluster.ClusterName, oldPhase, currentPhase)
		c.setPeeringPhase(remoteCluster.ClusterID, currentPhase)
	}

	currentNetEnabled := foreigncluster.IsNetworkingEnabled(&fc)
	if oldNetEnabled := c.getNetworkingEnabled(remoteCluster.ClusterID); oldNetEnabled != currentNetEnabled {
		klog.V(4).Infof("[%v] Networking enabled status changed: old: %v, new %v", remoteCluster.ClusterName, oldNetEnabled, currentNetEnabled)
		c.setNetworkingEnabled(remoteCluster.ClusterID, currentNetEnabled)
	}

	// Check if reflection towards the remote cluster has already been started.
	if _, found := c.Reflectors[remoteCluster.ClusterID]; found {
		return ctrl.Result{}, nil
	}

	if fc.Status.TenantNamespace.Local == "" || fc.Status.TenantNamespace.Remote == "" {
		klog.Infof("[%v] TenantNamespace is not yet set in resource %q", remoteCluster.ClusterName, fc.Name)
		return ctrl.Result{}, nil
	}
	config, err := c.IdentityReader.GetConfig(remoteCluster, fc.Status.TenantNamespace.Local)
	if err != nil {
		klog.Errorf("[%v] Unable to retrieve config from resource %q: %s", remoteCluster.ClusterName, fc.Name, err)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, c.setupReflectionToPeeringCluster(ctx, config, &fc)
}

// SetupWithManager registers a new controller for ForeignCluster resources.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	c.peeringPhases = make(map[string]consts.PeeringPhase)
	c.networkingEnabled = make(map[string]bool)

	resourceToBeProccesedPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).Named(operatorName).WithEventFilter(resourceToBeProccesedPredicate).
		For(&discoveryv1alpha1.ForeignCluster{}).
		Complete(c)
}

// ensureFinalizer updates the ForeignCluster to ensure the presence/absence of the finalizer.
func (c *Controller) ensureFinalizer(ctx context.Context, foreignCluster *discoveryv1alpha1.ForeignCluster,
	updater func(client.Object, string) bool) error {
	// Do not perform any action if the finalizer is already as expected
	if !updater(foreignCluster, finalizer) {
		return nil
	}

	return c.Client.Update(ctx, foreignCluster)
}

func (c *Controller) setupReflectionToPeeringCluster(ctx context.Context, config *rest.Config, fc *discoveryv1alpha1.ForeignCluster) error {
	remoteClusterID := fc.Spec.ClusterIdentity.ClusterID
	localNamespace := fc.Status.TenantNamespace.Local
	remoteNamespace := fc.Status.TenantNamespace.Remote

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Errorf("[%v] Unable to create dynamic client for remote cluster: %v", remoteClusterID, err)
		return err
	}

	reflector := c.ReflectionManager.NewForRemote(dynamicClient, remoteClusterID, localNamespace, remoteNamespace)
	reflector.Start(ctx)
	c.Reflectors[remoteClusterID] = reflector
	return nil
}

func (c *Controller) enforceReflectionStatus(ctx context.Context, remoteClusterID string, deleting bool) error {
	reflector, found := c.Reflectors[remoteClusterID]
	if !found {
		// The reflector object has not yet been setup
		return nil
	}

	phase := c.getPeeringPhase(remoteClusterID)
	networkingEnabled := c.getNetworkingEnabled(remoteClusterID)
	for i := range c.RegisteredResources {
		res := &c.RegisteredResources[i]
		if !deleting && isReplicationEnabled(phase, networkingEnabled, res) && !reflector.ResourceStarted(res) {
			reflector.StartForResource(ctx, res)
		} else if !isReplicationEnabled(phase, networkingEnabled, res) && reflector.ResourceStarted(res) {
			if err := reflector.StopForResource(res); err != nil {
				return err
			}
		}
	}

	return nil
}
