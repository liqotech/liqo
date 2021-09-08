// Copyright 2019-2021 The Liqo Authors
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

package liqonodeprovider

import (
	"context"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// LiqoNodeProvider is a node provider that manages the Liqo resources.
type LiqoNodeProvider struct {
	client    kubernetes.Interface
	dynClient dynamic.Interface

	node              *corev1.Node
	terminating       bool
	lastAppliedLabels map[string]string

	nodeName         string
	foreignClusterID string
	kubeletNamespace string
	resyncPeriod     time.Duration

	networkReady       bool
	podProviderStopper chan struct{}
	networkReadyChan   chan struct{}

	onNodeChangeCallback func(*corev1.Node)
	updateMutex          sync.Mutex
}

// Ping just implements the NodeProvider interface.
// It returns the error from the passed in context only.
func (p *LiqoNodeProvider) Ping(ctx context.Context) error {
	return ctx.Err()
}

// NotifyNodeStatus implements the NodeProvider interface.
func (p *LiqoNodeProvider) NotifyNodeStatus(ctx context.Context, f func(*corev1.Node)) {
	p.onNodeChangeCallback = f
}

// IsTerminating indicates if the node is in terminating (and in the draining phase).
func (p *LiqoNodeProvider) IsTerminating() bool {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()
	return p.terminating
}
