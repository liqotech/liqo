// Copyright 2019-2022 The Liqo Authors
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

package resourcerequestoperator

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

const (
	// requeueTimeout define a period of processed items requeue.
	requeueTimeout = 5 * time.Minute

	// maxRandom is used to generate a random delta to add to requeueTimeout to avoid syncing.
	maxRandom = 60
)

// OfferQueue is a component that periodically commands a broadcaster/broker to update its ResourceOffers.
// It also provides rate-limited retries for transient errors in the offering process.
type OfferQueue struct {
	// queue is the underlying generic queue
	queue workqueue.RateLimitingInterface
	// offerUpdater is the OfferUpdater that will create offers
	offerUpdater *OfferUpdater
	// identities maps cluster IDs (used by the queue) to ClusterIdentities
	identities map[string]discoveryv1alpha1.ClusterIdentity
}

// NewOfferQueue constructs an OfferQueue.
func NewOfferQueue(offerUpdater *OfferUpdater) OfferQueue {
	return OfferQueue{
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Offer update queue"),
		offerUpdater: offerUpdater,
		identities:   map[string]discoveryv1alpha1.ClusterIdentity{},
	}
}

// Start starts the update loop and blocks.
func (u *OfferQueue) Start(ctx context.Context) error {
	go wait.Until(u.run, 2*time.Second, ctx.Done())
	<-ctx.Done()
	u.queue.ShutDown()
	return nil // For compatibility with manager.Runnable
}

// Push pushes a cluster ID into the queue.
func (u *OfferQueue) Push(cluster discoveryv1alpha1.ClusterIdentity) {
	// The queue uses clusterID internally for disambiguation, so we store the identity separately
	// Note that we can't query Kubernetes for the identity on the fly when we consume the ID because we would need
	// a k8s client just for that
	u.queue.Add(cluster.ClusterID)
	u.identities[cluster.ClusterID] = cluster
}

// run processes items in the queue forever.
func (u *OfferQueue) run() {
	for u.processNextItem() {
	}
}

// processNextItem blocks on getting an item from the queue and processes it.
func (u *OfferQueue) processNextItem() bool {
	obj, shutdown := u.queue.Get()
	if shutdown {
		return false
	}
	cluster := u.identities[obj.(string)]

	klog.V(2).Infof("Processing cluster %s", cluster.ClusterName)
	requeue, err := u.offerUpdater.CreateOrUpdateOffer(cluster)
	if err != nil {
		klog.Errorf("Error processing cluster %s: %s", cluster.ClusterName, err)
		if requeue {
			// transient error: put the item back on the workqueue
			u.queue.AddRateLimited(cluster.ClusterID)
		} else {
			// permanent error (eg. the clusterID is no longer valid), do not requeue
			u.queue.Forget(cluster.ClusterID)
			u.offerUpdater.RemoveClusterID(cluster.ClusterID)
		}
	} else {
		// requeue after a random timeout
		u.queue.AddAfter(obj, getRandomTimeout())
	}
	u.queue.Done(obj)
	return true
}

// RemoveClusterID clears updates for the given cluster.
func (u *OfferQueue) RemoveClusterID(clusterID string) {
	u.queue.Forget(clusterID)
}

func getRandomTimeout() time.Duration {
	max := new(big.Int)
	max.SetInt64(int64(maxRandom))
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return requeueTimeout
	}
	return requeueTimeout + time.Duration(n.Int64())*time.Second
}
