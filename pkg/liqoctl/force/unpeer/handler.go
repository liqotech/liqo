// Copyright 2019-2026 The Liqo Authors
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

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/unauthenticate"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	mapsutil "github.com/liqotech/liqo/pkg/utils/maps"
)

// Options encapsulates the arguments of the force unpeer command.
type Options struct {
	*factory.Factory
	waiter *wait.Waiter

	ClusterID string
}

// NewOptions returns a new Options instance.
func NewOptions(localFactory *factory.Factory) *Options {
	return &Options{
		Factory: localFactory,
		waiter:  wait.NewWaiterFromFactory(localFactory),
	}
}

// RunForceUnpeer execute the `force unpeer` command.
func (o *Options) RunForceUnpeer(ctx context.Context) error {
	s := o.Printer.StartSpinner("Retrieving remote cluster info...")

	fc, err := fcutils.GetForeignClusterByID(ctx, o.CRClient, liqov1beta1.ClusterID(o.ClusterID))
	if k8sErrors.IsNotFound(err) {
		s.Fail("Remote cluster with ID %s not found", o.ClusterID)
		return fmt.Errorf("remote cluster with ID %s not found", o.ClusterID)
	} else if err != nil {
		s.Fail("Failed to get info about remote cluster")
		return fmt.Errorf("failed to get ForeignCluster by ID: %w", err)
	}

	s.Success("Info about the remote cluster retrieved successfully")
	s = o.Printer.StartSpinner("Declaring foreign cluster as permanently unreachable...")

	patch := client.MergeFrom(fc.DeepCopy())
	if fc.Annotations == nil {
		fc.Annotations = map[string]string{}
	}

	fc.SetAnnotations(
		mapsutil.Merge(fc.Annotations, map[string]string{
			consts.ForeignClusterPermanentlyUnreachableAnnotationKey: "true",
		}),
	)

	err = o.CRClient.Patch(ctx, fc, patch)
	if err != nil {
		s.Fail("Failed to declare ForeignCluster as permanently unreachable")
		return fmt.Errorf("failed to patch ForeignCluster with force-unpeer annotation: %w", err)
	}
	s.Success("Foreign cluster declared as permanently unreachable successfully")

	consumer := unauthenticate.NewCluster(o.Factory)

	// Delete tenant namespace on consumer cluster
	if err := consumer.DeleteTenantNamespace(ctx, liqov1beta1.ClusterID(o.ClusterID), true); err != nil {
		s.Fail("Failed to delete tenant namespace on consumer cluster")
		return fmt.Errorf("failed to delete tenant namespace on consumer cluster: %w", err)
	}

	o.Printer.Success.Println("Force unpeer executed successfully")

	return nil
}
