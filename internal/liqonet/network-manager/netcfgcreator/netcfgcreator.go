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

package netcfgcreator

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	"github.com/liqotech/liqo/pkg/utils/syncset"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
)

const componentName = "netcfgCreator"

// NetworkConfigCreator reconciles ForeignCluster objects to enforce the respective NetworkConfigs.
type NetworkConfigCreator struct {
	client.Client
	Scheme *runtime.Scheme

	foreignClusters *syncset.SyncSet
	secretWatcher   *SecretWatcher
	serviceWatcher  *ServiceWatcher

	PodCIDR      string
	ExternalCIDR string
}

// cluster-roles
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs,verbs=get;list;watch;create;update;patch;delete;deletecollection
// roles
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=services,verbs=get;list;watch

// Reconcile reconciles the state of ForeignCluster resources to enforce the respective NetworkConfigs.
func (ncc *NetworkConfigCreator) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Wait, in case the configuration has not completed yet.
	if !ncc.secretWatcher.WaitForConfigured(ctx) || !ncc.serviceWatcher.WaitForConfigured(ctx) {
		return ctrl.Result{}, errors.New("context expired before initialization completed")
	}

	klog.V(4).Infof("Reconciling ForeignCluster %q", req.Name)
	tracer := trace.New("Reconcile", trace.Field{Key: "ForeignCluster", Value: req.Name})
	ctx = trace.ContextWithTrace(ctx, tracer)
	defer tracer.LogIfLong(traceutils.LongThreshold())

	// Get the foreign cluster object.
	var fc discoveryv1alpha1.ForeignCluster
	if err := ncc.Get(ctx, req.NamespacedName, &fc); err != nil {
		// Remove the ForeignCluster from the list of known ones.
		ncc.foreignClusters.Remove(req.NamespacedName.Name)

		if !kerrors.IsNotFound(err) {
			klog.Errorf("Failed retrieving ForeignCluster: %v", err)
		}
		// Reconcile was triggered by a delete request.
		// No need to delete anything, as automatically collected by the owner reference.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if fc.Spec.ClusterIdentity.ClusterID == "" {
		klog.V(4).Infof("ForeignCluster %q not yet associated with a cluster ID", req.Name)
		return ctrl.Result{}, nil
	}

	if !foreigncluster.IsNetworkingEnabled(&fc) {
		klog.V(4).Infof("Networking for cluster %q is disabled, hence no need to create the networkconfig", req.Name)
	}

	// Add the ForeignCluster to the list of known ones.
	ncc.foreignClusters.Add(req.NamespacedName.Name)

	// A peering is (being) established and networking is enabled, hence we need to ensure the network interconnection.
	if fc.GetDeletionTimestamp().IsZero() && foreigncluster.IsNetworkingEnabled(&fc) &&
		(foreigncluster.IsIncomingJoined(&fc) || foreigncluster.IsOutgoingJoined(&fc)) {
		return ctrl.Result{}, ncc.EnforceNetworkConfigPresence(ctx, &fc)
	}

	// A peering is not established or the networking has been disabled, hence we need to tear down the network interconnection.
	return ctrl.Result{}, ncc.EnforceNetworkConfigAbsence(ctx, &fc)
}

// SetupWithManager registers a new controller for ForeignCluster resources.
func (ncc *NetworkConfigCreator) SetupWithManager(mgr ctrl.Manager) error {
	enqueuefn := func(rli workqueue.RateLimitingInterface) {
		ncc.foreignClusters.ForEach(func(fc string) { rli.Add(fc) })
	}

	ncc.foreignClusters = syncset.New()
	ncc.secretWatcher = NewSecretWatcher(enqueuefn)
	ncc.serviceWatcher = NewServiceWatcher(enqueuefn)

	localNetcfg, err := predicate.LabelSelectorPredicate(reflection.LocalResourcesLabelSelector())
	utilruntime.Must(err)

	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.ForeignCluster{}).
		Owns(&netv1alpha1.NetworkConfig{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}), localNetcfg)).
		Watches(&source.Kind{Type: &corev1.Secret{}}, ncc.secretWatcher.Handlers(), builder.WithPredicates(ncc.secretWatcher.Predicates())).
		Watches(&source.Kind{Type: &corev1.Service{}}, ncc.serviceWatcher.Handlers(), builder.WithPredicates(ncc.serviceWatcher.Predicates())).
		Complete(ncc)
}
