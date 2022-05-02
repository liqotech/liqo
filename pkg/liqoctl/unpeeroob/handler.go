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
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
)

const successfulMessage = `
Success 👌! You have correctly unpeered from cluster %q.
You can now:

* Check the status of the peering to see when it is completely disabled.
The field OutgoingPeering of the foreigncluster should be set to "None":

kubectl get foreignclusters %s
`

// Options encapsulates the arguments of the unpeer out-of-band command.
type Options struct {
	*factory.Factory

	ClusterName string
}

// Run implements the unpeer out-of-band command.
func (o *Options) Run(ctx context.Context) error {
	s := o.Printer.StartSpinner("Processing cluster unpeering")

	err := o.unpeer(ctx)
	if err != nil {
		s.Fail(err.Error())
		return err
	}
	s.Success("Cluster successfully unpeered")

	fmt.Printf(successfulMessage, o.ClusterName, o.ClusterName)
	return nil
}

func (o *Options) unpeer(ctx context.Context) error {
	var foreignCluster discoveryv1alpha1.ForeignCluster
	if err := o.CRClient.Get(ctx, types.NamespacedName{Name: o.ClusterName}, &foreignCluster); err != nil {
		return err
	}

	foreignCluster.Spec.OutgoingPeeringEnabled = discoveryv1alpha1.PeeringEnabledNo
	return o.CRClient.Update(ctx, &foreignCluster)
}
