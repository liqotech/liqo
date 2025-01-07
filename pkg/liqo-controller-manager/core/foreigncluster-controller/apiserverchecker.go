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

package foreignclustercontroller

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
)

const (
	apiServerCheckerFinalizer = "api-server-checker.liqo.io/finalizer"
)

// APIServerCheckers manage the foreign API server checker functions.
type APIServerCheckers struct {
	cancelFuncs  map[liqov1beta1.ClusterID]context.CancelFunc
	mutex        sync.RWMutex
	pingInterval time.Duration
	pingTimeout  time.Duration

	identityManager identitymanager.IdentityManager
}

// NewAPIServerCheckers returns a new APIServerCheckers struct.
func NewAPIServerCheckers(idManager identitymanager.IdentityManager, pingInterval, pingTimeout time.Duration) APIServerCheckers {
	return APIServerCheckers{
		cancelFuncs:  make(map[liqov1beta1.ClusterID]context.CancelFunc),
		mutex:        sync.RWMutex{},
		pingInterval: pingInterval,
		pingTimeout:  pingTimeout,

		identityManager: idManager,
	}
}

func (r *ForeignClusterReconciler) handleAPIServerChecker(ctx context.Context,
	foreignCluster *liqov1beta1.ForeignCluster) (cont bool, res ctrl.Result, err error) {
	r.APIServerCheckers.mutex.Lock()
	defer r.APIServerCheckers.mutex.Unlock()

	clusterID := foreignCluster.Spec.ClusterID

	needCondition := foreignCluster.Status.APIServerURL != "" || foreignCluster.Status.ForeignProxyURL != ""
	checkerDisabled := r.APIServerCheckers.pingInterval == 0 // checker is disabled if the ping interval is 0

	// If checker disabled, we consider the foreign API server as always ready.
	if needCondition && checkerDisabled {
		fcutils.EnsureGenericCondition(foreignCluster,
			liqov1beta1.APIServerStatusCondition,
			liqov1beta1.ConditionStatusEstablished,
			apiServerReadyReason,
			apiServerReadyMessage)

		// If the finalizer is present, remove it.
		if controllerutil.ContainsFinalizer(foreignCluster, apiServerCheckerFinalizer) {
			if err := r.ensureFinalizer(ctx, foreignCluster, controllerutil.RemoveFinalizer); err != nil {
				klog.Errorf("An error occurred while removing the finalizer from %q: %v", clusterID, err)
				return false, ctrl.Result{}, err
			}
			klog.Infof("Finalizer correctly removed from foreign cluster %q", clusterID)
			return false, ctrl.Result{Requeue: true}, nil
		}

		return true, ctrl.Result{}, nil
	}

	stopChecker, checkerExists := r.APIServerCheckers.cancelFuncs[clusterID]

	// If foreign API server checker not yet started:
	// - launch a new go routine for the checker with a new context
	// - add cancel context function to thread-safe map
	// - add finalizer to FC to prevent deletion if the routine is still running
	if needCondition && !checkerExists && foreignCluster.DeletionTimestamp.IsZero() {
		// Get the discovery client of the foreign cluster
		cfg, err := r.APIServerCheckers.identityManager.GetConfig(foreignCluster.Spec.ClusterID, foreignCluster.Status.TenantNamespace.Local)
		if err != nil {
			klog.Errorf("Error retrieving REST client of foreign cluster %q: %v", clusterID, err)
			return false, ctrl.Result{}, err
		}
		discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(cfg)

		// add the finalizer to the list and update it.
		if err := r.ensureFinalizer(ctx, foreignCluster, controllerutil.AddFinalizer); err != nil {
			klog.Errorf("An error occurred while adding the finalizer to %q: %v", clusterID, err)
			return false, ctrl.Result{}, err
		}
		klog.Infof("Finalizer correctly added to foreign cluster %q", clusterID)

		klog.Infof("Starting API server checker for foreign cluster %q", clusterID)
		var contextChecker context.Context
		contextChecker, stopChecker = context.WithCancel(context.Background())
		r.APIServerCheckers.cancelFuncs[clusterID] = stopChecker
		go r.runAPIServerChecker(contextChecker, clusterID, discoveryClient)

		return false, ctrl.Result{Requeue: true}, nil
	}

	// If FC is being deleted:
	// - stop the go routine of the checker with the cancel context function
	// - delete the FC entry from the thread safe map
	// - remove finalizer from FC to allow deletion
	if !needCondition || !foreignCluster.DeletionTimestamp.IsZero() {
		if checkerExists {
			klog.Infof("Stopping API server checker for foreign cluster %q", clusterID)
			stopChecker()
			delete(r.APIServerCheckers.cancelFuncs, clusterID)
		}

		fcutils.DeleteGenericCondition(foreignCluster, liqov1beta1.APIServerStatusCondition)

		// remove the finalizer from the list and update it.
		if controllerutil.ContainsFinalizer(foreignCluster, apiServerCheckerFinalizer) {
			if err := r.ensureFinalizer(ctx, foreignCluster, controllerutil.RemoveFinalizer); err != nil {
				klog.Errorf("An error occurred while removing the finalizer from %q: %v", clusterID, err)
				return false, ctrl.Result{}, err
			}
			klog.Infof("Finalizer correctly removed from foreign cluster %q", clusterID)
			return false, ctrl.Result{Requeue: true}, nil
		}
	}

	// Nothing to do, continue the reconciliation
	return true, ctrl.Result{}, nil
}

// ensureFinalizer updates the ForeignCluster to ensure the presence/absence of the API server checker finalizer.
func (r *ForeignClusterReconciler) ensureFinalizer(ctx context.Context, foreignCluster *liqov1beta1.ForeignCluster,
	updater func(client.Object, string) bool) error {
	// Do not perform any action if the finalizer is already as expected
	if !updater(foreignCluster, apiServerCheckerFinalizer) {
		return nil
	}

	return r.Client.Update(ctx, foreignCluster)
}

func (r *ForeignClusterReconciler) runAPIServerChecker(ctx context.Context, clusterID liqov1beta1.ClusterID,
	discoveryClient *discovery.DiscoveryClient) {
	var oldStatus, newStatus liqov1beta1.ConditionStatusType
	var reason, message string

	// We delay for a bit to not update the foreign cluster too soon, avoiding possible collision with the
	// foreign cluster reconciliation
	time.Sleep(2 * time.Second)

	checkAndUpdateCallback := func(ctx context.Context) (done bool, err error) {
		klog.V(5).Infof("[%s] checking foreign API server readiness", clusterID)

		fc, err := fcutils.GetForeignClusterByID(ctx, r.Client, clusterID)
		if err != nil {
			klog.Errorf("[%s] error while getting foreign cluster: %v", clusterID, err)
			return false, nil
		}

		oldStatus = fcutils.GetAPIServerStatus(fc)

		if r.isForeignAPIServerReady(ctx, discoveryClient, clusterID) {
			newStatus = liqov1beta1.ConditionStatusEstablished
			reason = apiServerReadyReason
			message = apiServerReadyMessage
		} else {
			newStatus = liqov1beta1.ConditionStatusError
			reason = apiServerNotReadyReason
			message = apiServerNotReadyMessage
		}

		if oldStatus != newStatus {
			fcutils.EnsureGenericCondition(fc,
				liqov1beta1.APIServerStatusCondition, newStatus, reason, message)
			if err := r.Status().Update(ctx, fc); err != nil {
				klog.Errorf("[%s] error while updating foreign API server status: %v", clusterID, err)
				return false, nil
			}
			klog.Infof("[%s] updated foreign API server status (from %s to %s)", clusterID, oldStatus, newStatus)
		}

		return false, nil
	}

	klog.Infof("[%s] foreign API server readiness checker started", clusterID)

	// Ignore errors because only caused by context cancellation.
	_ = wait.PollUntilContextCancel(ctx, r.APIServerCheckers.pingInterval, true, checkAndUpdateCallback)

	klog.Infof("[%s] foreign API server readiness checker stopped", clusterID)
}

func (r *ForeignClusterReconciler) isForeignAPIServerReady(ctx context.Context, discoveryClient *discovery.DiscoveryClient,
	id liqov1beta1.ClusterID) bool {
	pingCtx, cancel := context.WithTimeout(ctx, r.APIServerCheckers.pingTimeout)
	defer cancel()

	start := time.Now()

	_, err := discoveryClient.RESTClient().Get().AbsPath("/livez").DoRaw(pingCtx)
	if err != nil {
		klog.Errorf("[%s] foreign API server readiness check failed: %v", id, err)
		return false
	}

	klog.V(5).Infof("[%s] foreign API server readiness check completed successfully in %v", id, time.Since(start))
	return true
}
