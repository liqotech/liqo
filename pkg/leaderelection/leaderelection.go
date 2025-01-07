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
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const leaderLabel = "leaderelection.liqo.io/leader"

var (
	lock                 sync.RWMutex
	leading              = false
	addToSchemeFunctions = []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		coordv1.AddToScheme,
	}
)

// PodInfo contains the information about the pod.
type PodInfo struct {
	PodName        string
	Namespace      string
	DeploymentName *string
}

// Opts contains the options to configure the leader election mechanism.
type Opts struct {
	PodInfo
	Client            client.Client
	LeaderElectorName string
	LeaseDuration     time.Duration
	RenewDeadline     time.Duration
	RetryPeriod       time.Duration
	InitCallback      func()
	StopCallback      func()
	LabelLeader       bool
}

// Init initializes the leader election mechanism.
func Init(opts *Opts, rc *rest.Config, eb record.EventBroadcaster) (*leaderelection.LeaderElector, error) {
	// Adds the APIs to the scheme.
	scheme := runtime.NewScheme()
	for _, addToScheme := range addToSchemeFunctions {
		if err := addToScheme(scheme); err != nil {
			return nil, fmt.Errorf("unable to add scheme: %w", err)
		}
	}

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
			OnStartedLeading: func(ctx context.Context) {
				lock.Lock()
				defer lock.Unlock()
				klog.Infof("Leader election: this pod is the leader")
				if opts.LabelLeader {
					if err := handleLeaderLabel(ctx, rc, scheme, &opts.PodInfo); err != nil {
						klog.Errorf("Leader election: unable to handle labeling of leader: %s", err)
						os.Exit(1)
					}
				}
				leading = true
				if opts.InitCallback != nil {
					opts.InitCallback()
				}
			},
			OnStoppedLeading: func() {
				lock.Lock()
				defer lock.Unlock()
				if opts.StopCallback != nil {
					opts.StopCallback()
				}
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
		Name:            opts.LeaderElectorName,
		ReleaseOnCancel: true,
	})
	if err != nil {
		return nil, err
	}

	return leaderelector, nil
}

// Run run the leader election mechanism.
func Run(ctx context.Context, leaderelector *leaderelection.LeaderElector) {
	klog.Info("Leader election: starting leader election")
	leaderelector.Run(ctx)
	// If the context is not canceled, the leader election terminated unexpectedly.
	if ctx.Err() == nil {
		utilruntime.Must(fmt.Errorf("leader election terminated"))
	}
}

// IsLeader returns true if the current pod is the leader.
func IsLeader() bool {
	lock.RLock()
	defer lock.RUnlock()
	return leading
}

// handleLeaderLabel labels the current pod as leader and unlabels eventual old leader.
func handleLeaderLabel(ctx context.Context, rc *rest.Config, scheme *runtime.Scheme, opts *PodInfo) error {
	klog.Infof("Leader election: labeling this pod as leader and unlabeling eventual old leader")
	if opts.DeploymentName == nil {
		return fmt.Errorf("deployment name not specified")
	}

	cl, err := client.New(rc, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("unable to create client: %w", err)
	}

	return handleLeaderLabelWithClient(ctx, cl, opts)
}

// handleLeaderLabelWithClient labels the current pod as leader and unlabels eventual old leader using the given client.
func handleLeaderLabelWithClient(ctx context.Context, cl client.Client, opts *PodInfo) error {
	var deployment appsv1.Deployment
	if err := cl.Get(ctx, client.ObjectKey{
		Namespace: opts.Namespace,
		Name:      *opts.DeploymentName,
	}, &deployment); err != nil {
		return fmt.Errorf("unable to get deployment: %w", err)
	}

	podsFromDepSelector := client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)}
	var podList corev1.PodList
	if err := cl.List(ctx, &podList, client.InNamespace(deployment.Namespace), podsFromDepSelector); err != nil {
		return fmt.Errorf("unable to list pods of deployment %s/%s: %w", deployment.Namespace, deployment.Name, err)
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Name == opts.PodName {
			// Label pod if it is the new leader.
			if pod.Labels == nil {
				pod.Labels = map[string]string{}
			}
			pod.Labels[leaderLabel] = "true"
			if err := cl.Update(ctx, pod); err != nil {
				return fmt.Errorf("unable to label pod %s/%s: %w", pod.Namespace, pod.Name, err)
			}
			klog.Infof("Leader election: pod %s/%s labeled as leader", pod.Namespace, pod.Name)
		} else {
			// Unlabel pod if it is the old leader.
			value, ok := pod.Labels[leaderLabel]
			if ok && !strings.EqualFold(value, "false") {
				delete(pod.Labels, leaderLabel)
				if err := cl.Update(ctx, pod); err != nil {
					return fmt.Errorf("unable to remove label from pod %s/%s: %w", pod.Namespace, pod.Name, err)
				}
				klog.Infof("Leader election: pod %s/%s unlabeled as leader", pod.Namespace, pod.Name)
			}
		}
	}

	return nil
}
