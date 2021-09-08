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

package resourcerequestoperator

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller/interfaces"
)

// requeueTimeout define a period of processed items requeue.
const requeueTimeout = 5 * time.Minute

// maxRandom is used to generate a random delta to add to requeueTimeout to avoid syncing.
const maxRandom = 60

// OfferUpdater is a component which wraps all ResourceOffer update logic.
type OfferUpdater struct {
	queue workqueue.RateLimitingInterface
	client.Client
	broadcasterInt interfaces.ClusterResourceInterface
	homeClusterID  string
	clusterLabels  map[string]string
	scheme         *runtime.Scheme
}

// Setup initializes all parameters of the OfferUpdater component.
func (u *OfferUpdater) Setup(clusterID string, scheme *runtime.Scheme, broadcaster interfaces.ClusterResourceInterface,
	k8Client client.Client, clusterLabels map[string]string) {
	u.broadcasterInt = broadcaster
	u.Client = k8Client
	u.homeClusterID = clusterID
	u.scheme = scheme
	u.clusterLabels = clusterLabels
	u.queue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Offer update queue")
}

// Start runs the OfferUpdate worker.
func (u *OfferUpdater) Start(ctx context.Context, group *sync.WaitGroup) {
	defer u.queue.ShutDown()
	defer group.Done()
	go u.startRunner(ctx)
	<-ctx.Done()
}

func (u *OfferUpdater) startRunner(ctx context.Context) {
	wait.Until(u.run, 2*time.Second, ctx.Done())
}

func (u *OfferUpdater) run() {
	for u.processNextItem() {
	}
}

func (u *OfferUpdater) processNextItem() bool {
	obj, shutdown := u.queue.Get()
	if shutdown {
		return false
	}
	err := func(obj interface{}) error {
		defer u.queue.Done(obj)
		var clusterID string
		var ok bool
		if clusterID, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			u.queue.Forget(obj)
			return fmt.Errorf("error getting object %v from OfferUpater queue. It is not a string", obj)
		}
		// call createOrUpdate which after some controls generate a new resourceOffer for this clusterID or update it if exists.
		if requeue, err := u.createOrUpdateOffer(clusterID); err != nil {
			if requeue {
				// requeue is true due to a transient error so put the item back on the workqueue.
				u.queue.AddRateLimited(clusterID)
			} else {
				// requeue == false means that the clusterID is no more valid and so it will be not requeued.
				u.Remove(clusterID)
				u.broadcasterInt.RemoveClusterID(clusterID)
			}
			return fmt.Errorf("error during updating ResourceOffer for cluster %s: %w", clusterID, err)
		}
		return nil
	}(obj)
	if err != nil {
		klog.Errorf("Error occurred during ResourceOffer update %s", err)
		return true
	}
	klog.Infof("Update cluster %s processed", obj.(string))

	// requeue after timeout seconds
	u.queue.AddAfter(obj, getRandomTimeout())
	return true
}

func (u *OfferUpdater) createOrUpdateOffer(clusterID string) (bool, error) {
	list, err := u.getResourceRequest(clusterID)
	if err != nil {
		return true, err
	} else if len(list.Items) != 1 {
		// invalid clusterID so return requeue = false. The clusterID will be removed from the workqueue and broadacaster maps.
		return false, fmt.Errorf("ClusterID %s is no more valid. Deleting", clusterID)
	}
	request := list.Items[0]
	resources := u.broadcasterInt.ReadResources(clusterID)
	offer := &sharingv1alpha1.ResourceOffer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: request.GetNamespace(),
			Name:      offerPrefix + u.homeClusterID,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.Background(), u.Client, offer, func() error {
		if offer.Labels != nil {
			offer.Labels[discovery.ClusterIDLabel] = request.Spec.ClusterIdentity.ClusterID
			offer.Labels[crdreplicator.LocalLabelSelector] = "true"
			offer.Labels[crdreplicator.DestinationLabel] = request.Spec.ClusterIdentity.ClusterID
		} else {
			offer.Labels = map[string]string{
				discovery.ClusterIDLabel:         request.Spec.ClusterIdentity.ClusterID,
				crdreplicator.LocalLabelSelector: "true",
				crdreplicator.DestinationLabel:   request.Spec.ClusterIdentity.ClusterID,
			}
		}
		offer.Spec.ClusterId = u.homeClusterID
		offer.Spec.ResourceQuota.Hard = resources.DeepCopy()
		offer.Spec.Labels = u.clusterLabels
		return controllerutil.SetControllerReference(&request, offer, u.scheme)
	})

	if err != nil {
		klog.Error(err)
		return true, err
	}
	klog.Infof("%s -> %s Offer: %s/%s", u.homeClusterID, op, offer.Namespace, offer.Name)
	return true, nil
}

func (u *OfferUpdater) getResourceRequest(clusterID string) (*discoveryv1alpha1.ResourceRequestList, error) {
	resourceRequestList := &discoveryv1alpha1.ResourceRequestList{}
	err := u.Client.List(context.Background(), resourceRequestList, client.MatchingLabels{
		crdreplicator.RemoteLabelSelector: clusterID,
	})
	if err != nil {
		return nil, err
	}
	return resourceRequestList, nil
}

// Push add new clusterID to update queue which will be processes as soon as possible.
func (u *OfferUpdater) Push(clusterID string) {
	u.queue.Add(clusterID)
}

// Remove removes a specified clusterID from the update queue and it will be no more processed.
func (u *OfferUpdater) Remove(clusterID string) {
	u.queue.Forget(clusterID)
	klog.Infof("Removed cluster %s from update queue", clusterID)
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
