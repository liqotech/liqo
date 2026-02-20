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

package foreignclustercontroller

import (
	"bytes"
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

// apiServerCheckerEntry holds the state for a running API server checker goroutine.
type apiServerCheckerEntry struct {
	cancel   context.CancelFunc
	certData []byte // client certificate data used by the checker, to detect renewals
}

// APIServerCheckers manage the foreign API server checker functions.
type APIServerCheckers struct {
	checkers     map[liqov1beta1.ClusterID]*apiServerCheckerEntry
	mutex        sync.RWMutex
	pingInterval time.Duration
	pingTimeout  time.Duration

	identityManager identitymanager.IdentityManager
}

// NewAPIServerCheckers returns a new APIServerCheckers struct.
func NewAPIServerCheckers(idManager identitymanager.IdentityManager, pingInterval, pingTimeout time.Duration) APIServerCheckers {
	return APIServerCheckers{
		checkers:     make(map[liqov1beta1.ClusterID]*apiServerCheckerEntry),
		mutex:        sync.RWMutex{},
		pingInterval: pingInterval,
		pingTimeout:  pingTimeout,

		identityManager: idManager,
	}
}

func (r *ForeignClusterReconciler) handleAPIServerChecker(ctx context.Context,
	foreignCluster *liqov1beta1.ForeignCluster) (cont bool, res ctrl.Result, err error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	clusterID := foreignCluster.Spec.ClusterID

	needCondition := foreignCluster.Status.APIServerURL != "" || foreignCluster.Status.ForeignProxyURL != ""
	checkerDisabled := r.pingInterval == 0 // checker is disabled if the ping interval is 0

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

	checker, checkerExists := r.checkers[clusterID]

	// If FC is being deleted or no longer needs the condition:
	// - stop the checker goroutine
	// - delete the entry from the map
	// - remove finalizer from FC to allow deletion
	if !needCondition || !foreignCluster.DeletionTimestamp.IsZero() {
		if checkerExists {
			klog.Infof("Stopping API server checker for foreign cluster %q", clusterID)
			checker.cancel()
			delete(r.checkers, clusterID)
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

		return true, ctrl.Result{}, nil
	}

	// Get the current config for the foreign cluster.
	cfg, err := r.identityManager.GetConfig(foreignCluster.Spec.ClusterID, foreignCluster.Status.TenantNamespace.Local)
	if err != nil {
		klog.Errorf("Error retrieving REST client of foreign cluster %q: %v", clusterID, err)
		return false, ctrl.Result{}, err
	}

	// If the checker is running but the certificate has changed (renewed),
	// stop the current checker so a new one is started with the updated config.
	if checkerExists && !bytes.Equal(checker.certData, cfg.CertData) {
		klog.Infof("Certificate changed for foreign cluster %q, restarting API server checker", clusterID)
		checker.cancel()
		delete(r.checkers, clusterID)
		checkerExists = false
	}

	// Start a new checker if one is not running.
	if !checkerExists {
		discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(cfg)

		// add the finalizer to the list and update it.
		if err := r.ensureFinalizer(ctx, foreignCluster, controllerutil.AddFinalizer); err != nil {
			klog.Errorf("An error occurred while adding the finalizer to %q: %v", clusterID, err)
			return false, ctrl.Result{}, err
		}
		klog.Infof("Finalizer correctly added to foreign cluster %q", clusterID)

		klog.Infof("Starting API server checker for foreign cluster %q", clusterID)
		contextChecker, stopChecker := context.WithCancel(context.Background())
		r.checkers[clusterID] = &apiServerCheckerEntry{
			cancel:   stopChecker,
			certData: cfg.CertData,
		}
		go r.runAPIServerChecker(contextChecker, clusterID, discoveryClient)

		return false, ctrl.Result{Requeue: true}, nil
	}

	// Checker is already running.
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
	_ = wait.PollUntilContextCancel(ctx, r.pingInterval, true, checkAndUpdateCallback)

	klog.Infof("[%s] foreign API server readiness checker stopped", clusterID)
}

func (r *ForeignClusterReconciler) isForeignAPIServerReady(ctx context.Context, discoveryClient *discovery.DiscoveryClient,
	id liqov1beta1.ClusterID) bool {
	pingCtx, cancel := context.WithTimeout(ctx, r.pingTimeout)
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
