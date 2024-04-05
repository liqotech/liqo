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

package crdreplicator

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/internal/crdReplicator/resources"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
)

const (
	operatorName = "crdReplicator-operator"
	finalizer    = "crdreplicator.liqo.io/operator"
)

// Controller reconciles identity Secrets to start/stop the reflection of registered resources to remote clusters.
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
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;update;patch
// role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=configmaps,verbs=get;list;watch

// identity management
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile handles requests for subscribed types of object.
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	var prefix string
	tracer := trace.New("Reconcile", trace.Field{Key: "Secret", Value: req.Name})
	defer tracer.LogIfLong(traceutils.LongThreshold())

	var secret corev1.Secret
	if err := c.Get(ctx, req.NamespacedName, &secret); err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("secret %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("unable to get secret %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	klog.Infof("Processing Secret %q", req.NamespacedName)

	// Validate the secret to ensure it is a control plane identity and it has all the required labels and annotations.
	if err := c.validateSecret(&secret); err != nil {
		klog.Errorf("secret %q is not valid: %v", req.NamespacedName, err)
		return ctrl.Result{}, nil
	}

	// Extract remote cluster informations from the secret
	remoteClusterID := secret.Labels[consts.RemoteClusterID]
	localTenantNamespace := secret.Namespace
	remoteTenantNamespace := secret.Annotations[consts.RemoteTenantNamespaceAnnotKey]
	if remoteClusterName, ok := secret.Annotations[consts.RemoteClusterName]; ok { // optional, only used for logging
		prefix = fmt.Sprintf("[%s] ", remoteClusterName)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if !secret.ObjectMeta.DeletionTimestamp.IsZero() {
		// the object is being deleted
		if controllerutil.ContainsFinalizer(&secret, finalizer) {
			// close remote watcher for remote cluster
			reflector, ok := c.Reflectors[remoteClusterID]
			if ok {
				if err := reflector.Stop(); err != nil {
					klog.Errorf("%sFailed to stop reflection: %v", prefix, err)
					return ctrl.Result{}, err
				}
				delete(c.Reflectors, remoteClusterID)
			}

			// remove the finalizer from the list and update it.
			if err := c.ensureFinalizer(ctx, &secret, controllerutil.RemoveFinalizer); err != nil {
				klog.Errorf("An error occurred while removing the finalizer to %q: %v", req.NamespacedName, err)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	// Defer the function to start/stop the reflection of the different resources based on the peering status.
	defer func() {
		if err == nil {
			err = c.enforceReflectionStatus(ctx, remoteClusterID, !secret.GetDeletionTimestamp().IsZero())
		}
	}()

	// TODO: redefine peering phases.
	currentPhase := consts.PeeringPhaseAuthenticated
	// The remote identity is not yet available, hence it is not possible to continue.
	if currentPhase == consts.PeeringPhaseNone {
		klog.Infof("%sPeering phase not yet available for cluster %s", prefix, remoteClusterID)
		return ctrl.Result{}, nil
	}

	// Add the finalizer to ensure the reflection is correctly stopped
	if err := c.ensureFinalizer(ctx, &secret, controllerutil.AddFinalizer); err != nil {
		klog.Errorf("An error occurred while adding the finalizer to %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if oldPhase := c.getPeeringPhase(remoteClusterID); oldPhase != currentPhase {
		klog.V(4).Infof("%sPeering phase changed: old: %v, new: %v", prefix, oldPhase, currentPhase)
		c.setPeeringPhase(remoteClusterID, currentPhase)
	}

	// TODO: replication of NetworkConfig is not necessary with the new network. Refactor to support new networking module.
	currentNetEnabled := true
	if oldNetEnabled := c.getNetworkingEnabled(remoteClusterID); oldNetEnabled != currentNetEnabled {
		klog.V(4).Infof("%sNetworking enabled status changed: old: %v, new %v", prefix, oldNetEnabled, currentNetEnabled)
		c.setNetworkingEnabled(remoteClusterID, currentNetEnabled)
	}

	// Check if reflection towards the remote cluster has already been started.
	if _, found := c.Reflectors[remoteClusterID]; found {
		return ctrl.Result{}, nil
	}

	config, err := c.IdentityReader.GetConfig(v1alpha1.ClusterIdentity{ClusterID: remoteClusterID}, localTenantNamespace)
	if err != nil {
		klog.Errorf("%sUnable to retrieve config for clusterID %q: %v", prefix, remoteClusterID, err)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, c.setupReflectionToPeeringCluster(ctx, config, remoteClusterID, localTenantNamespace, remoteTenantNamespace)
}

// SetupWithManager registers a new controller for identity Secrets.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	c.peeringPhases = make(map[string]consts.PeeringPhase)
	c.networkingEnabled = make(map[string]bool)

	resourceToBeProccesedPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}

	secretsFilter, err := predicate.LabelSelectorPredicate(c.controlPlaneIdentitySecretLabelSelector())
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(operatorName).
		WithEventFilter(resourceToBeProccesedPredicate).
		For(&corev1.Secret{}, builder.WithPredicates(secretsFilter)).
		Complete(c)
}

func (c *Controller) controlPlaneIdentitySecretLabelSelector() metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      consts.RemoteClusterID,
				Operator: metav1.LabelSelectorOpExists,
			},
			{
				Key:      consts.IdentityTypeLabelKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{string(authv1alpha1.ControlPlaneIdentityType)},
			},
		},
	}
}

func (c *Controller) validateSecret(secret *corev1.Secret) error {
	nsName := client.ObjectKeyFromObject(secret)

	if secret.Labels == nil || secret.Annotations == nil {
		return fmt.Errorf("secret %q does not have the required labels and annotations", nsName)
	}

	if idType, ok := secret.Labels[consts.IdentityTypeLabelKey]; ok && idType != string(authv1alpha1.ControlPlaneIdentityType) {
		return fmt.Errorf("secret %q does not contain a control plane identity", nsName)
	}

	if _, ok := secret.Labels[consts.RemoteClusterID]; !ok {
		return fmt.Errorf("secret %q does not have the remote cluster ID label %s", nsName, consts.RemoteClusterID)
	}

	if _, ok := secret.Annotations[consts.RemoteTenantNamespaceAnnotKey]; !ok {
		return fmt.Errorf("secret %q does not have the remote tenant namespace annotation key %s", nsName, consts.RemoteTenantNamespaceAnnotKey)
	}

	return nil
}

// ensureFinalizer updates the identity secret to ensure the presence/absence of the finalizer.
func (c *Controller) ensureFinalizer(ctx context.Context, secret *corev1.Secret,
	updater func(client.Object, string) bool) error {
	// Do not perform any action if the finalizer is already as expected
	if !updater(secret, finalizer) {
		return nil
	}

	return c.Client.Update(ctx, secret)
}

func (c *Controller) setupReflectionToPeeringCluster(ctx context.Context, config *rest.Config,
	remoteClusterID, localNamespace, remoteNamespace string) error {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Errorf("%sUnable to create dynamic client for remote cluster: %v", remoteClusterID, err)
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
