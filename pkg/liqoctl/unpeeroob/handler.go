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

package unpeeroob

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/types"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
)

// Options encapsulates the arguments of the unpeer out-of-band command.
type Options struct {
	*factory.Factory

	ClusterName string
	Timeout     time.Duration
}

// Run implements the unpeer out-of-band command.
func (o *Options) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	s := o.Printer.StartSpinner("Processing cluster unpeering")

	fc, err := o.unpeer(ctx)
	if err != nil {
		s.Fail("Failed unpeering clusters: ", output.PrettyErr(err))
		return err
	}
	s.Success("Peering disabled")

	if err = o.wait(ctx, &fc.Spec.ClusterIdentity); err != nil {
		return err
	}

	o.Printer.Success.Println("Peering successfully removed")
	return nil
}

func (o *Options) unpeer(ctx context.Context) (*discoveryv1alpha1.ForeignCluster, error) {
	var foreignCluster discoveryv1alpha1.ForeignCluster
	if err := o.CRClient.Get(ctx, types.NamespacedName{Name: o.ClusterName}, &foreignCluster); err != nil {
		return nil, err
	}

	foreignCluster.Spec.OutgoingPeeringEnabled = discoveryv1alpha1.PeeringEnabledNo
	if err := o.CRClient.Update(ctx, &foreignCluster); err != nil {
		return nil, err
	}
	return &foreignCluster, nil
}

func (o *Options) wait(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	waiter := wait.NewWaiterFromFactory(o.Factory)
	return waiter.ForOutgoingUnpeering(ctx, remoteClusterID)
}
