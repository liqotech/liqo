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

package crdreplicator

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/foreigncluster"
)

// ForeignClusterStateController reconciles on the state of the foreign clusters to manager the reflection.
type ForeignClusterStateController struct {
	client.Client

	// Reflectors is a map containing the reflectors towards each remote cluster.
	Reflectors map[liqov1beta1.ClusterID]*reflection.Reflector
}

// cluster-role
// +kubebuilder:rbac:groups=core.liqo.io,resources=foreignclusters,verbs=get;list;watch

// Reconcile reconciles the state of the foreign clusters to manage the reflection.
func (c *ForeignClusterStateController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the ForeignCluster object
	foreignCluster := &liqov1beta1.ForeignCluster{}
	if err := c.Get(ctx, req.NamespacedName, foreignCluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// If the foreign cluster seems to be disabled, skip reconciliation.
	if foreignCluster.Status.Role == liqov1beta1.UnknownRole {
		klog.Infof("ForeignCluster %q has unknown role, skipping reconciliation", foreignCluster.Name)
		return ctrl.Result{}, nil
	}

	// Get the reflector for the remote cluster
	reflector, exists := c.Reflectors[foreignCluster.Spec.ClusterID]
	if !exists {
		klog.Warningf("No reflector found for ForeignCluster %q, will retry later", foreignCluster.Name)
		return ctrl.Result{}, fmt.Errorf("no reflector found for ForeignCluster %q", foreignCluster.Name)
	}

	if dead, message := foreigncluster.IsDead(foreignCluster); dead {
		reflector.RemoteReachable.Store(false)
		klog.Warningf("Remote cluster %q is dead: %s", foreignCluster.Name, message)
	} else {
		reflector.RemoteReachable.Store(true)
		klog.Infof("Remote cluster %q is alive", foreignCluster.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers a new controller for identity Secrets.
func (c *ForeignClusterStateController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlForeignClusterStateCRDReplicator).
		For(&liqov1beta1.ForeignCluster{}).
		Complete(c)
}
