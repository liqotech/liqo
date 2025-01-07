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

package liqonodeprovider

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
)

// LiqoNodeProvider is a node provider that manages the Liqo resources.
type LiqoNodeProvider struct {
	localClient           kubernetes.Interface
	remoteDiscoveryClient discovery.DiscoveryInterface
	dynClient             dynamic.Interface

	node                   *corev1.Node
	terminating            bool
	lastAppliedLabels      map[string]string
	lastAppliedAnnotations map[string]string
	lastAppliedTaints      []corev1.Taint

	nodeName           string
	nodeIP             string
	foreignClusterID   liqov1beta1.ClusterID
	tenantNamespace    string
	resyncPeriod       time.Duration
	pingDisabled       bool
	checkNetworkStatus bool

	networkModuleEnabled bool
	networkReady         bool

	onNodeChangeCallback func(*corev1.Node)
	updateMutex          sync.Mutex
}

// Ping checks if the node is still active.
func (p *LiqoNodeProvider) Ping(ctx context.Context) error {
	if p.pingDisabled {
		return nil
	}

	var err error

	start := time.Now()
	klog.V(4).Infof("Checking whether the remote API server is ready")

	// Get the foreigncluster using the given clusterID
	fc, err := foreignclusterutils.GetForeignClusterByIDWithDynamicClient(ctx, p.dynClient, p.foreignClusterID)
	if apierrors.IsNotFound(err) {
		// If the foreigncluster is not found, ping directly the remote API server as fallback.
		err = p.pingWithClient(ctx)
	} else if err == nil {
		cond := foreignclusterutils.GetAPIServerStatus(fc)
		switch {
		case cond == liqov1beta1.ConditionStatusNone:
			err = p.pingWithClient(ctx)
		case cond != liqov1beta1.ConditionStatusEstablished:
			err = fmt.Errorf("API server is not ready")
		}
	}

	if err != nil {
		return fmt.Errorf("[%s] API server readiness check failed: %w", p.foreignClusterID, err)
	}

	klog.V(4).Infof("[%s] API server readiness check completed successfully in %v", p.foreignClusterID, time.Since(start))

	return nil
}

// NotifyNodeStatus implements the NodeProvider interface.
func (p *LiqoNodeProvider) NotifyNodeStatus(_ context.Context, f func(*corev1.Node)) {
	p.onNodeChangeCallback = f
}

// IsTerminating indicates if the node is in terminating (and in the draining phase).
func (p *LiqoNodeProvider) IsTerminating() bool {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()
	return p.terminating
}

// GetNode returns the node managed by the provider.
func (p *LiqoNodeProvider) GetNode() *corev1.Node {
	return p.node
}

func (p *LiqoNodeProvider) pingWithClient(ctx context.Context) error {
	_, err := p.remoteDiscoveryClient.RESTClient().Get().AbsPath("/livez").DoRaw(ctx)
	if err != nil {
		return err
	}
	return nil
}
