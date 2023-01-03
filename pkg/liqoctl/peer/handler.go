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

package peer

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
)

// Options encapsulates the arguments of the peer command.
type Options struct {
	*factory.Factory

	ClusterName string
	Timeout     time.Duration
}

// Run implements the peer out-of-band command.
func (o *Options) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	s := o.Printer.StartSpinner("Processing cluster peering")

	remoteClusterID, err := o.peer(ctx)
	if err != nil {
		s.Fail(err.Error())
		return err
	}
	s.Success("Peering enabled")

	if err = o.Wait(ctx, remoteClusterID); err != nil {
		return err
	}

	o.Printer.Success.Println("Peering successfully established")
	return nil
}

func (o *Options) peer(ctx context.Context) (*discoveryv1alpha1.ClusterIdentity, error) {
	var fc discoveryv1alpha1.ForeignCluster
	if err := o.CRClient.Get(ctx, types.NamespacedName{
		Name: o.ClusterName,
	}, &fc); err != nil {
		return nil, err
	}

	fc.Spec.OutgoingPeeringEnabled = discoveryv1alpha1.PeeringEnabledYes

	return &fc.Spec.ClusterIdentity, retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return o.CRClient.Update(ctx, &fc)
	})
}

// Wait waits for the peering to the remote cluster to be fully enabled.
func (o *Options) Wait(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	waiter := wait.NewWaiterFromFactory(o.Factory)

	if err := waiter.ForAuth(ctx, remoteClusterID); err != nil {
		return err
	}

	if err := waiter.ForOutgoingPeering(ctx, remoteClusterID); err != nil {
		return err
	}

	if err := waiter.ForNetwork(ctx, remoteClusterID); err != nil {
		return err
	}

	return waiter.ForNode(ctx, remoteClusterID)
}
