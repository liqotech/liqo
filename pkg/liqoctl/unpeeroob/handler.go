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

package unpeeroob

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
)

// Options encapsulates the arguments of the unpeer out-of-band command.
type Options struct {
	*factory.Factory

	ClusterName string
	Timeout     time.Duration

	// Whether to enforce the peering to be of type out-of-band, and delete the ForeignCluster resource.
	UnpeerOOBMode bool
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
	s.Success("Outgoing peering marked as disabled")

	if err = o.wait(ctx, &fc.Spec.ClusterIdentity); err != nil {
		return err
	}

	// Do not attempt to delete the ForeignCluster resource if the unpeer command is not in OOB mode.
	if !o.UnpeerOOBMode {
		return nil
	}

	s = o.Printer.StartSpinner("Removing the foreign cluster resource")
	if deleted, err := o.delete(ctx, fc); err != nil {
		s.Fail("Failed removing the foreign cluster resource: ", output.PrettyErr(err))
		return err
	} else if !deleted {
		s.Warning("The foreign cluster resource was not removed, as an incoming peering is still active")
		s.Warning(fmt.Sprintf("Issue the unpeer command on the remote cluster %q to disable it", o.ClusterName))
		return nil
	}

	o.Printer.Success.Println("Foreign cluster resource successfully removed")
	return nil
}

func (o *Options) unpeer(ctx context.Context) (*discoveryv1alpha1.ForeignCluster, error) {
	var foreignCluster discoveryv1alpha1.ForeignCluster
	if err := o.CRClient.Get(ctx, types.NamespacedName{Name: o.ClusterName}, &foreignCluster); err != nil {
		return nil, err
	}

	// Do not proceed if the peering is not out-of-band and that mode is set.
	if o.UnpeerOOBMode && foreignCluster.Spec.PeeringType != discoveryv1alpha1.PeeringTypeOutOfBand {
		return nil, fmt.Errorf("the peering type towards remote cluster %q is %s, expected %s",
			o.ClusterName, foreignCluster.Spec.PeeringType, discoveryv1alpha1.PeeringTypeOutOfBand)
	}

	foreignCluster.Spec.OutgoingPeeringEnabled = discoveryv1alpha1.PeeringEnabledNo
	if err := o.CRClient.Update(ctx, &foreignCluster); err != nil {
		return nil, err
	}
	return &foreignCluster, nil
}

func (o *Options) delete(ctx context.Context, fc *discoveryv1alpha1.ForeignCluster) (deleted bool, err error) {
	incoming := peeringconditionsutils.GetStatus(fc, discoveryv1alpha1.IncomingPeeringCondition)
	if incoming != discoveryv1alpha1.PeeringConditionStatusNone {
		return false, nil
	}

	if err := o.Factory.CRClient.Delete(ctx, fc); err != nil {
		return true, client.IgnoreNotFound(err)
	}

	return true, nil
}

func (o *Options) wait(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	waiter := wait.NewWaiterFromFactory(o.Factory)
	return waiter.ForOutgoingUnpeering(ctx, remoteClusterID)
}
