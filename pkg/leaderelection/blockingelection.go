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

package leaderelection

import (
	"context"
	"fmt"
	"os"
	"time"

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

// Blocking runs the blocking leader election.
func Blocking(ctx context.Context, rc *rest.Config, eb record.EventBroadcaster, opts *Opts) (bool, error) {
	// Adds the APIs to the scheme.
	scheme := runtime.NewScheme()
	for _, addToScheme := range addToSchemeFunctions {
		if err := addToScheme(scheme); err != nil {
			return false, fmt.Errorf("unable to add scheme: %w", err)
		}
	}

	elected := make(chan struct{})

	leaderelector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock: &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      opts.LeaderElectorName,
				Namespace: opts.Namespace,
			},
			Client: coordinationv1.NewForConfigOrDie(rc),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity:      opts.PodName,
				EventRecorder: eb.NewRecorder(scheme, corev1.EventSource{Component: opts.PodName}),
			},
		},
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				lock.Lock()
				defer lock.Unlock()
				klog.Infof("Leader election: this pod is the leader")
				if opts.LabelLeader && opts.Client != nil {
					if err := handleLeaderLabelWithClient(ctx, opts.Client, &opts.PodInfo); err != nil {
						klog.Errorf("Failed to label leader pod: %v", err)
						os.Exit(1)
					}
				}
				close(elected)
			},
			OnStoppedLeading: func() {
				lock.Lock()
				defer lock.Unlock()
				klog.Infof("Leader election: this pod is not the leader anymore")
				os.Exit(1)
			},
			OnNewLeader: func(identity string) {
				klog.Infof("Leader election: %s is the current leader", identity)
			},
		},
		LeaseDuration:   opts.LeaseDuration,
		RenewDeadline:   opts.RenewDeadline,
		RetryPeriod:     opts.RetryPeriod,
		Name:            opts.LeaderElectorName,
		ReleaseOnCancel: true,
	})
	if err != nil {
		return false, err
	}

	go leaderelector.Run(ctx)

	for {
		select {
		case <-ctx.Done():
			return false, nil
		case <-elected:
			return true, nil
		case <-time.After(30 * time.Second):
			klog.V(3).Infof("Waiting to be elected as leader...")
		}
	}
}
