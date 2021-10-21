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
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
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

type ReflectorSet struct {
	// Reflector that reads on the local tenant ns and writes on the remote tenant ns
	TenantToTenant *reflection.Reflector
	// Reflector that reads on the local tenant ns and writes on the remote public ns
	TenantToPublic *reflection.Reflector
	// Reflector that reads on the local public ns and writes on the remote tenant ns
	PublicToTenant *reflection.Reflector
}

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
	Reflectors map[string]ReflectorSet

	// IdentityReader is an interface to manage remote identities, and to get the rest config.
	IdentityReader identitymanager.IdentityReader

	peeringPhases      map[string]consts.PeeringPhase
	peeringPhasesMutex sync.RWMutex
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

	remoteClusterID := fc.Spec.ClusterIdentity.ClusterID
	klog.Infof("[%v] Processing ForeignCluster %q", remoteClusterID, fc.Name)
	// Prevent issues in case the remote cluster ID has not yet been set
	if remoteClusterID == "" {
		klog.Infof("Remote Cluster ID is not yet set in resource %q", fc.Name)
		return ctrl.Result{}, nil
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if !fc.ObjectMeta.DeletionTimestamp.IsZero() {
		// the object is being deleted
		if controllerutil.ContainsFinalizer(&fc, finalizer) {
			// close remote watcher for remote cluster
			reflectorSet, ok := c.Reflectors[remoteClusterID]
			if ok {
				if err := reflectorSet.PublicToTenant.Stop(); err != nil {
					klog.Errorf("[%v] Failed to stop reflection: %v", remoteClusterID, err)
					return ctrl.Result{}, err
				}
				if err := reflectorSet.TenantToPublic.Stop(); err != nil {
					klog.Errorf("[%v] Failed to stop reflection: %v", remoteClusterID, err)
					return ctrl.Result{}, err
				}
				if err := reflectorSet.TenantToTenant.Stop(); err != nil {
					klog.Errorf("[%v] Failed to stop reflection: %v", remoteClusterID, err)
					return ctrl.Result{}, err
				}
				delete(c.Reflectors, remoteClusterID)
			}

			// remove the finalizer from the list and update it.
			if err := c.ensureFinalizer(ctx, &fc, false, controllerutil.RemoveFinalizer); err != nil {
				klog.Errorf("An error occurred while removing the finalizer to %q: %s", fc.Name, err)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	// Defer the function to start/stop the reflection of the different resources based on the peering status.
	defer func() {
		if err == nil {
			err = c.enforceReflectionStatus(ctx, remoteClusterID, !fc.ObjectMeta.DeletionTimestamp.IsZero())
		}
	}()

	currentPhase := foreigncluster.GetPeeringPhase(&fc)
	// The remote identity is not yet available, hence it is not possible to continue.
	if currentPhase == consts.PeeringPhaseNone {
		klog.Infof("Foreign cluster %s not yet associated with a valid identity", fc.Name)
		return ctrl.Result{}, nil
	}

	// Add the finalizer to ensure the reflection is correctly stopped
	if err := c.ensureFinalizer(ctx, &fc, true, controllerutil.AddFinalizer); err != nil {
		klog.Errorf("An error occurred while adding the finalizer to %q: %s", fc.Name, err)
		return ctrl.Result{}, err
	}

	if oldPhase := c.getPeeringPhase(remoteClusterID); oldPhase != currentPhase {
		klog.V(4).Infof("[%v] Peering phase changed: old: %v, new: %v", remoteClusterID, oldPhase, currentPhase)
		c.setPeeringPhase(remoteClusterID, currentPhase)
	}

	// Check if reflection towards the remote cluster has already been started.
	if _, found := c.Reflectors[remoteClusterID]; found {
		return ctrl.Result{}, nil
	}

	if fc.Status.TenantNamespace.Local == "" || fc.Status.TenantNamespace.Remote == "" {
		klog.Infof("[%v] TenantNamespace is not yet set in resource %q", remoteClusterID, fc.Name)
		return ctrl.Result{}, nil
	}
	var config *rest.Config
	if fc.Spec.InducedPeering.InducedPeeringEnabled == discoveryv1alpha1.PeeringEnabledYes {
		config, err = c.IdentityReader.GetConfig(fc.Spec.InducedPeering.OriginClusterIdentity.ClusterID, fc.Status.TenantNamespace.Local)
		if err != nil {
			klog.Errorf("[%v] Unable to retrieve config from resource %q: %s", remoteClusterID, fc.Name, err)
			return ctrl.Result{}, nil
		}
	} else {
		config, err = c.IdentityReader.GetConfig(remoteClusterID, fc.Status.TenantNamespace.Local)
		if err != nil {
			klog.Errorf("[%v] Unable to retrieve config from resource %q: %s", remoteClusterID, fc.Name, err)
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, c.setupReflectionToPeeringCluster(ctx, config, &fc)
}

// SetupWithManager registers a new controller for ForeignCluster resources.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
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
	expected bool, updater func(client.Object, string)) error {
	// Do not perform any action if the finalizer is already absent
	if controllerutil.ContainsFinalizer(foreignCluster, finalizer) == expected {
		return nil
	}

	updater(foreignCluster, finalizer)
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
	passthrough := fc.Spec.InducedPeering.InducedPeeringEnabled == discoveryv1alpha1.PeeringEnabledYes
	reflectorSet := ReflectorSet{
		TenantToTenant: c.ReflectionManager.NewForRemote(dynamicClient, remoteClusterID, localNamespace, remoteNamespace),
		TenantToPublic: c.ReflectionManager.NewForRemote(dynamicClient, remoteClusterID, localNamespace, consts.LiqoPublicNS),
		PublicToTenant: c.ReflectionManager.NewForRemote(dynamicClient, remoteClusterID, consts.LiqoPublicNS, remoteNamespace),
	}

	if passthrough {
		reflectorSet.TenantToPublic.Start(ctx)
	} else {
		reflectorSet.PublicToTenant.Start(ctx)
		reflectorSet.TenantToTenant.Start(ctx)
	}

	c.Reflectors[remoteClusterID] = reflectorSet
	return nil
}

func (c *Controller) enforceReflectionStatus(ctx context.Context, remoteClusterID string, deleting bool) error {
	reflectorSet, found := c.Reflectors[remoteClusterID]
	if !found {
		// The reflector object has not yet been setup
		return nil
	}

	phase := c.getPeeringPhase(remoteClusterID)
	for i := range c.RegisteredResources {
		res := &c.RegisteredResources[i]
		if reflectorSet.PublicToTenant != nil && res.GroupVersionResource == netv1alpha1.NetworkConfigGroupVersionResource {
			if !deleting && isReplicationEnabled(phase, res) && !reflectorSet.PublicToTenant.ResourceStarted(res) {
				reflectorSet.PublicToTenant.StartForResource(ctx, res)
			} else if !isReplicationEnabled(phase, res) && reflectorSet.PublicToTenant.ResourceStarted(res) {
				if err := reflectorSet.PublicToTenant.StopForResource(res); err != nil {
					return err
				}
			}
		}
		if reflectorSet.TenantToTenant != nil && phase != consts.PeeringPhaseInduced {
			if !deleting && isReplicationEnabled(phase, res) && !reflectorSet.TenantToTenant.ResourceStarted(res) {
				reflectorSet.TenantToTenant.StartForResource(ctx, res)
			} else if !isReplicationEnabled(phase, res) && reflectorSet.TenantToTenant.ResourceStarted(res) {
				if err := reflectorSet.TenantToTenant.StopForResource(res); err != nil {
					return err
				}
			}
		}
		if reflectorSet.TenantToPublic != nil && res.GroupVersionResource == netv1alpha1.NetworkConfigGroupVersionResource && phase == consts.PeeringPhaseInduced {
			if !deleting && isReplicationEnabled(phase, res) && !reflectorSet.TenantToPublic.ResourceStarted(res) {
				reflectorSet.TenantToPublic.StartForResource(ctx, res)
			} else if !isReplicationEnabled(phase, res) && reflectorSet.TenantToPublic.ResourceStarted(res) {
				if err := reflectorSet.TenantToPublic.StopForResource(res); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
