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

package crdreplicator

import (
	"context"
	"crypto/sha256"
	"fmt"

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

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/internal/crdReplicator/resources"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/utils/getters"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
)

const (
	finalizer = "crdreplicator.liqo.io/operator"
)

// Controller reconciles identity Secrets to start/stop the reflection of registered resources to remote clusters.
type Controller struct {
	Scheme *runtime.Scheme
	client.Client
	ClusterID liqov1beta1.ClusterID

	// RegisteredResources is a list of GVRs of resources to be replicated.
	RegisteredResources []resources.Resource

	// ReflectionManager is the object managing the reflection towards remote clusters.
	ReflectionManager *reflection.Manager
	// Reflectors is a map containing the reflectors towards each remote cluster.
	Reflectors map[liqov1beta1.ClusterID]*reflection.Reflector

	// IdentityReader is an interface to manage remote identities, and to get the rest config.
	IdentityReader identitymanager.IdentityReader
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
	remoteClusterID := liqov1beta1.ClusterID(secret.Labels[consts.RemoteClusterID])
	localTenantNamespace := secret.Namespace
	remoteTenantNamespace := secret.Annotations[consts.RemoteTenantNamespaceAnnotKey]

	// examine DeletionTimestamp to determine if object is under deletion
	if !secret.ObjectMeta.DeletionTimestamp.IsZero() {
		// the object is being deleted
		if controllerutil.ContainsFinalizer(&secret, finalizer) {
			// close remote watcher for remote cluster
			if err := c.stopReflector(remoteClusterID, false); err != nil {
				klog.Errorf("%sFailed to stop reflection: %v", prefix, err)
				return ctrl.Result{}, err
			}

			// remove the finalizer from the list and update it.
			if err := c.ensureFinalizer(ctx, &secret, controllerutil.RemoveFinalizer); err != nil {
				klog.Errorf("An error occurred while removing the finalizer to %q: %v", req.NamespacedName, err)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	// Defer the function to start/stop the reflection of the different resources.
	defer func() {
		if err == nil {
			err = c.enforceReflectionStatus(ctx, remoteClusterID, !secret.GetDeletionTimestamp().IsZero())
		}
	}()

	// Add the finalizer to ensure the reflection is correctly stopped
	if err := c.ensureFinalizer(ctx, &secret, controllerutil.AddFinalizer); err != nil {
		klog.Errorf("An error occurred while adding the finalizer to %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Check if reflection towards the remote cluster has already been started.
	if reflector, found := c.Reflectors[remoteClusterID]; found {
		// We ignore the case in which the secret lacks of the kubeconfig, as in that case we still want to delete the reflector
		// and manage the error.
		secretContent := secret.Data[consts.KubeconfigSecretField]
		secretHash := c.hashSecretConfig(secretContent)

		// If there are no changes on the secret or on the remote namespace where the reflector operats, skip reconciliation.
		if reflector.GetSecretHash() == secretHash && reflector.GetRemoteTenantNamespace() == remoteTenantNamespace {
			return ctrl.Result{}, nil
		}

		// If there have been a change on the secret, delete the secret to allow the creation of a new reflector.
		klog.Infof("%sChanges detected on the control plane secret %q for clusterID %q: recreating reflector",
			prefix, req.NamespacedName, remoteClusterID)
		// Stop the reflection to update the reflector
		if err := c.stopReflector(remoteClusterID, true); err != nil {
			klog.Errorf("%sFailed to stop reflection: %v", prefix, err)
			return ctrl.Result{}, err
		}
	}

	// We need to get the secret to make sure that there are not multiple secrets pointing to the same cluster
	currSecret, err := getters.GetControlPlaneKubeconfigSecretByClusterID(ctx, c.Client, remoteClusterID)
	if err != nil {
		klog.Errorf("%sUnable to process secret for clusterID %q: %v", prefix, remoteClusterID, err)
		return ctrl.Result{}, nil
	}

	config, err := c.IdentityReader.GetConfigFromSecret(remoteClusterID, currSecret)
	if err != nil {
		klog.Errorf("%sUnable to retrieve config for clusterID %q: %v", prefix, remoteClusterID, err)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, c.setupReflectionToPeeringCluster(ctx, currSecret, config, remoteClusterID, localTenantNamespace, remoteTenantNamespace)
}

// SetupWithManager registers a new controller for identity Secrets.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	resourceToBeProccesedPredicate := predicate.Funcs{
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
	}

	secretsFilter, err := predicate.LabelSelectorPredicate(c.controlPlaneIdentitySecretLabelSelector())
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlSecretCRDReplicator).
		For(&corev1.Secret{}, builder.WithPredicates(secretsFilter)).
		WithEventFilter(resourceToBeProccesedPredicate).
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
				Values:   []string{string(authv1beta1.ControlPlaneIdentityType)},
			},
		},
	}
}

func (c *Controller) validateSecret(secret *corev1.Secret) error {
	nsName := client.ObjectKeyFromObject(secret)

	if secret.Labels == nil || secret.Annotations == nil {
		return fmt.Errorf("secret %q does not have the required labels and annotations", nsName)
	}

	if idType, ok := secret.Labels[consts.IdentityTypeLabelKey]; ok && idType != string(authv1beta1.ControlPlaneIdentityType) {
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

func (c *Controller) setupReflectionToPeeringCluster(ctx context.Context, secret *corev1.Secret, config *rest.Config,
	remoteClusterID liqov1beta1.ClusterID, localNamespace, remoteNamespace string) error {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Errorf("%sUnable to create dynamic client for remote cluster: %v", remoteClusterID, err)
		return err
	}

	secretHash := c.hashSecretConfig(secret.Data[consts.KubeconfigSecretField])

	reflector := c.ReflectionManager.NewForRemote(dynamicClient, remoteClusterID, localNamespace, remoteNamespace, secretHash)
	reflector.Start(ctx)
	c.Reflectors[remoteClusterID] = reflector
	return nil
}

func (c *Controller) stopReflector(remoteClusterID liqov1beta1.ClusterID, skipChecks bool) error {
	reflector, ok := c.Reflectors[remoteClusterID]
	if ok {
		stopFn := reflector.Stop
		// Use the StopForce function if we want to skip the checks
		if skipChecks {
			stopFn = reflector.StopForce
		}

		if err := stopFn(); err != nil {
			return err
		}
		delete(c.Reflectors, remoteClusterID)
	}
	return nil
}

func (c *Controller) hashSecretConfig(secretData []byte) string {
	hash := sha256.Sum256(secretData)
	return fmt.Sprintf("%x", hash)
}

func (c *Controller) enforceReflectionStatus(ctx context.Context, remoteClusterID liqov1beta1.ClusterID, deleting bool) error {
	reflector, found := c.Reflectors[remoteClusterID]
	if !found {
		// The reflector object has not yet been setup
		return nil
	}

	for i := range c.RegisteredResources {
		res := &c.RegisteredResources[i]
		if !deleting && !reflector.ResourceStarted(res) {
			reflector.StartForResource(ctx, res)
		} else if deleting && reflector.ResourceStarted(res) {
			if err := reflector.StopForResource(res); err != nil {
				return err
			}
		}
	}

	return nil
}
