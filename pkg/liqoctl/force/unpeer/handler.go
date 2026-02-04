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
//

package unpeer

import (
	"context"
	"fmt"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/unauthenticate"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Options encapsulates the arguments of the info command.
type Options struct {
	*factory.Factory
	waiter *wait.Waiter

	ClusterId string
}

func NewOptions(localFactory *factory.Factory) *Options {
	return &Options{
		Factory: localFactory,
		waiter:  wait.NewWaiterFromFactory(localFactory),
	}
}

// RunUnpeerInfo execute the `force unpeer` command.
func (o *Options) RunUnpeerForce(ctx context.Context) error {

	s := o.Printer.StartSpinner("Checking ForeignCluster existence")

	fc := &liqov1beta1.ForeignCluster{}
	err := o.CRClient.Get(ctx, client.ObjectKey{
		Name: string(o.ClusterId),
	}, fc)

	if err != nil {
		s.Fail("Error while retrieving ForeignCluster: ", output.PrettyErr(err))
		return err
	}

	s.Success("ForeignCluster found")

	patch := client.MergeFrom(fc.DeepCopy())
	if fc.Annotations == nil {
		fc.Annotations = map[string]string{}
	}

	client.Object.SetAnnotations(fc, map[string]string{
		"liqo.io/foreign-cluster-permanently-unreachable": "true",
	})

	err = o.CRClient.Patch(ctx, fc, patch)
	if err != nil {
		return fmt.Errorf("failed to patch ForeignCluster with force-unpeer annotation: %w", err)
	}

	consumer := unauthenticate.NewCluster(o.Factory)

	// Delete tenant namespace on consumer cluster
	if err := consumer.DeleteTenantNamespace(ctx, liqov1beta1.ClusterID(o.ClusterId), true); err != nil {
		return err
	}

	s.Success("Force unpeer executed successfully")

	return nil

}
