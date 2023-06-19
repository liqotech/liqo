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
	"sync"
	"time"

	cv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

var (
	lock    sync.RWMutex
	wait    sync.WaitGroup
	leading = false
)

// InitAndRun initializes and runs the leader election mechanism.
func InitAndRun(ns string, rc *rest.Config, id string, eb record.EventBroadcaster) error {
	scheme := runtime.NewScheme()
	// TODO controllo quale schema va aggiunto.
	err := cv1.AddToScheme(scheme)
	if err != nil {
		klog.Error(err)
		return err
	}
	wait.Add(1)
	leaderelector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock: &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      LeaderElectorName,
				Namespace: ns,
			},
			Client: coordinationv1.NewForConfigOrDie(rc),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity:      id,
				EventRecorder: eb.NewRecorder(scheme, corev1.EventSource{Component: id}),
			},
		},
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				lock.Lock()
				klog.Infof("Leader election: this pod is the leader", id)
				leading = true
				lock.Unlock()
			},
			OnStoppedLeading: func() {
				lock.Lock()
				klog.Infof("Leader election: this pod is not the leader anymore", id)
				leading = false
				lock.Unlock()
			},
			OnNewLeader: func(identity string) {
				klog.Infof("Leader election: %s is the current leader", identity)
				if identity == id {
					lock.Lock()
					leading = true
					lock.Unlock()
				}
				wait.Done()
			},
		},
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Name:          LeaderElectorName,
	})
	if err != nil {
		return err
	}
	go leaderelector.Run(context.Background())
	wait.Wait()
	return nil
}

// IsLeader returns true if the current virtual node is the leader.
func IsLeader() bool {
	lock.RLock()
	defer lock.RUnlock()
	return leading
}
