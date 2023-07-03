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

package leaderelection

import (
	"context"
	"fmt"
	"sync"
	"time"

	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

var (
	lock    sync.RWMutex
	leading = false
)

// Opts contains the options to configure the leader election mechanism.
type Opts struct {
	Enabled         bool
	PodName         string
	TenantNamespace string
	LeaseDuration   time.Duration
	RenewDeadline   time.Duration
	RetryPeriod     time.Duration
}

// InitAndRun initializes and runs the leader election mechanism.
func InitAndRun(ctx context.Context, opts Opts, rc *rest.Config,
	eb record.EventBroadcaster, initCallback func()) error {
	if !opts.Enabled {
		return nil
	}
	scheme := runtime.NewScheme()
	err := coordv1.AddToScheme(scheme)
	if err != nil {
		klog.Error(err)
		return err
	}
	leaderelector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock: &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      LeaderElectorName,
				Namespace: opts.TenantNamespace,
			},
			Client: coordinationv1.NewForConfigOrDie(rc),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity:      opts.PodName,
				EventRecorder: eb.NewRecorder(scheme, corev1.EventSource{Component: opts.PodName}),
			},
		},
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				lock.Lock()
				defer lock.Unlock()
				klog.Infof("Leader election: this pod is the leader")
				leading = true
				initCallback()
			},
			OnStoppedLeading: func() {
				lock.Lock()
				defer lock.Unlock()
				klog.Infof("Leader election: this pod is not the leader anymore")
				leading = false
			},
			OnNewLeader: func(identity string) {
				klog.Infof("Leader election: %s is the current leader", identity)
			},
		},
		LeaseDuration:   opts.LeaseDuration,
		RenewDeadline:   opts.RenewDeadline,
		RetryPeriod:     opts.RetryPeriod,
		Name:            LeaderElectorName,
		ReleaseOnCancel: true,
	})
	if err != nil {
		return err
	}
	go func() {
		klog.Info("Leader election: starting leader election")
		leaderelector.Run(ctx)
		// If the context is not canceled, the leader election terminated unexpectedly.
		if ctx.Err() == nil {
			utilruntime.Must(fmt.Errorf("leader election terminated"))
		}
	}()
	return nil
}

// IsLeader returns true if the current virtual node is the leader.
func IsLeader() bool {
	lock.RLock()
	defer lock.RUnlock()
	return leading
}
