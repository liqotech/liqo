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

package foreignclusteroperator

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
)

const (
	apiServerCheckerFinalizer = "APIServerChecker.liqo.io"
)

// APIServerCheckers manage the foreign API server checker functions.
type APIServerCheckers struct {
	cancelFuncs  map[string]context.CancelFunc
	mutex        sync.RWMutex
	pingInterval time.Duration
	pingTimeout  time.Duration
}

// NewAPIServerCheckers returns a new APIServerCheckers struct.
func NewAPIServerCheckers(pingInterval, pingTimeout time.Duration) APIServerCheckers {
	return APIServerCheckers{
		cancelFuncs:  make(map[string]context.CancelFunc),
		mutex:        sync.RWMutex{},
		pingInterval: pingInterval,
		pingTimeout:  pingTimeout,
	}
}

func (r *ForeignClusterReconciler) handleAPIServerChecker(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) (cont bool, res ctrl.Result, err error) {
	r.APIServerCheckers.mutex.Lock()
	defer r.APIServerCheckers.mutex.Unlock()

	clusterName := foreignCluster.Spec.ClusterIdentity.ClusterName
	clusterID := foreignCluster.Spec.ClusterIdentity.ClusterID
	checkerDisabled := r.APIServerCheckers.pingInterval == 0 // checker is disabled if the ping interval is 0

	// If checker disabled, we consider the foreign API server as always ready.
	if checkerDisabled {
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.APIServerStatusCondition,
			discoveryv1alpha1.PeeringConditionStatusEstablished,
			apiServerReadyReason,
			apiServerReadyMessage)

		// If the finalizer is present, remove it.
		if controllerutil.ContainsFinalizer(foreignCluster, apiServerCheckerFinalizer) {
			if err := r.ensureFinalizer(ctx, foreignCluster, controllerutil.RemoveFinalizer); err != nil {
				klog.Errorf("An error occurred while removing the finalizer from %q: %v", clusterName, err)
				return false, ctrl.Result{}, err
			}
			klog.Infof("Finalizer correctly removed from foreign cluster %q", clusterName)
			return false, ctrl.Result{Requeue: true}, nil
		}

		return true, ctrl.Result{}, nil
	}

	stopChecker, checkerExists := r.APIServerCheckers.cancelFuncs[clusterID]

	// If foreign API server checker not yet started:
	// - launch a new go routine for the checker with a new context
	// - add cancel context function to thread-safe map
	// - add finalizer to FC to prevent deletion if the routine is still running
	if !checkerExists && foreignCluster.DeletionTimestamp.IsZero() {
		// Get the discovery client of the foreign cluster
		cfg, err := r.IdentityManager.GetConfig(foreignCluster.Spec.ClusterIdentity, foreignCluster.Status.TenantNamespace.Local)
		if err != nil {
			klog.Errorf("Error retrieving REST client of foreign cluster %q: %v", clusterName, err)
			return false, ctrl.Result{}, err
		}
		discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(cfg)

		// add the finalizer to the list and update it.
		if err := r.ensureFinalizer(ctx, foreignCluster, controllerutil.AddFinalizer); err != nil {
			klog.Errorf("An error occurred while adding the finalizer to %q: %v", clusterName, err)
			return false, ctrl.Result{}, err
		}
		klog.Infof("Finalizer correctly added to foreign cluster %q", clusterName)

		klog.Infof("Starting API server checker for foreign cluster %q", clusterName)
		var contextChecker context.Context
		contextChecker, stopChecker = context.WithCancel(context.Background())
		r.APIServerCheckers.cancelFuncs[clusterID] = stopChecker
		go r.runAPIServerChecker(contextChecker, clusterName, discoveryClient)

		return false, ctrl.Result{Requeue: true}, nil
	}

	// If FC is being deleted:
	// - stop the go routine of the checker with the cancel context function
	// - delete the FC entry from the thread safe map
	// - remove finalizer from FC to allow deletion
	if !foreignCluster.DeletionTimestamp.IsZero() &&
		(checkerExists || controllerutil.ContainsFinalizer(foreignCluster, apiServerCheckerFinalizer)) {
		if checkerExists {
			klog.Infof("Stopping API server checker for foreign cluster %q", clusterName)
			stopChecker()
			delete(r.APIServerCheckers.cancelFuncs, clusterID)
		}

		// remove the finalizer from the list and update it.
		if err := r.ensureFinalizer(ctx, foreignCluster, controllerutil.RemoveFinalizer); err != nil {
			klog.Errorf("An error occurred while removing the finalizer from %q: %v", clusterName, err)
			return false, ctrl.Result{}, err
		}
		klog.Infof("Finalizer correctly removed from foreign cluster %q", clusterName)

		return false, ctrl.Result{Requeue: true}, nil
	}

	// Nothing to do, continue the reconciliation
	return true, ctrl.Result{}, nil
}

// ensureFinalizer updates the ForeignCluster to ensure the presence/absence of the API server checker finalizer.
func (r *ForeignClusterReconciler) ensureFinalizer(ctx context.Context, foreignCluster *discoveryv1alpha1.ForeignCluster,
	updater func(client.Object, string) bool) error {
	// Do not perform any action if the finalizer is already as expected
	if !updater(foreignCluster, apiServerCheckerFinalizer) {
		return nil
	}

	return r.Client.Update(ctx, foreignCluster)
}

func (r *ForeignClusterReconciler) runAPIServerChecker(ctx context.Context, clusterName string,
	discoveryClient *discovery.DiscoveryClient) {
	var fc = new(discoveryv1alpha1.ForeignCluster)
	var oldStatus, newStatus discoveryv1alpha1.PeeringConditionStatusType
	var reason, message string

	// We delay for a bit to not update the foreign cluster too soon, avoiding possible collision with the
	// foreign cluster reconciliation
	time.Sleep(2 * time.Second)

	checkAndUpdateCallback := func(ctx context.Context) (done bool, err error) {
		klog.V(5).Infof("[%s] checking foreign API server readiness", clusterName)

		if r.Get(ctx, types.NamespacedName{Name: clusterName}, fc) != nil {
			klog.Errorf("[%s] ForeignCluster not found", clusterName)
			return false, nil
		}

		oldStatus = peeringconditionsutils.GetStatus(fc, discoveryv1alpha1.APIServerStatusCondition)

		if r.isForeignAPIServerReady(ctx, discoveryClient, clusterName) {
			newStatus = discoveryv1alpha1.PeeringConditionStatusEstablished
			reason = apiServerReadyReason
			message = apiServerReadyMessage
		} else {
			newStatus = discoveryv1alpha1.PeeringConditionStatusError
			reason = apiServerNotReadyReason
			message = apiServerNotReadyMessage
		}

		if oldStatus != newStatus {
			peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.APIServerStatusCondition, newStatus, reason, message)
			if err := r.Status().Update(ctx, fc); err != nil {
				klog.Errorf("[%s] error while updating foreign API server status: %v", clusterName, err)
				return false, nil
			}
			klog.Infof("[%s] updated foreign API server status (from %s to %s)", clusterName, oldStatus, newStatus)
		}

		return false, nil
	}

	klog.Infof("[%s] foreign API server readiness checker started", clusterName)

	// Ignore errors because only caused by context cancellation.
	_ = wait.PollImmediateInfiniteWithContext(ctx, r.APIServerCheckers.pingInterval, checkAndUpdateCallback)

	klog.Infof("[%s] foreign API server readiness checker stopped", clusterName)
}

func (r *ForeignClusterReconciler) isForeignAPIServerReady(ctx context.Context, discoveryClient *discovery.DiscoveryClient, name string) bool {
	pingCtx, cancel := context.WithTimeout(ctx, r.APIServerCheckers.pingTimeout)
	defer cancel()

	start := time.Now()

	_, err := discoveryClient.RESTClient().Get().AbsPath("/livez").DoRaw(pingCtx)
	if err != nil {
		klog.Errorf("[%s] foreign API server readiness check failed: %v", name, err)
		return false
	}

	klog.V(5).Infof("[%s] foreign API server readiness check completed successfully in %v", name, time.Since(start))
	return true
}
