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

package remove

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/common"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// ClusterArgs encapsulates arguments required to disable an outgoing peering to a remote cluster.
type ClusterArgs struct {
	ClusterName string
	ClusterID   string
}

// HandleRemoveCommand handles the remove command, configuring all the resources required to disable an outgoing peering.
func HandleRemoveCommand(ctx context.Context, t *ClusterArgs) error {
	restConfig, err := common.GetLiqoctlRestConf()
	if err != nil {
		return err
	}

	k8sClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}

	err = processRemoveCluster(ctx, t, k8sClient)
	if err != nil {
		return err
	}

	err = printSuccessfulOutputMessage(ctx, t, k8sClient)
	if err != nil {
		return err
	}

	return nil
}

func printSuccessfulOutputMessage(ctx context.Context, t *ClusterArgs, k8sClient client.Client) error {
	fc, err := foreigncluster.GetForeignClusterByID(ctx, k8sClient, t.ClusterID)
	if err != nil {
		return err
	}
	fmt.Printf(SuccessfulMessage, t.ClusterName, fc.Name)
	return nil
}

func processRemoveCluster(ctx context.Context, t *ClusterArgs, k8sClient client.Client) error {
	err := enforceForeignCluster(ctx, k8sClient, t)
	if err != nil {
		return err
	}
	return nil
}

func enforceForeignCluster(ctx context.Context, cl client.Client, t *ClusterArgs) error {
	// Get ForeignCluster
	var foreignCluster discoveryv1alpha1.ForeignCluster
	if err := cl.Get(ctx, types.NamespacedName{Name: t.ClusterName}, &foreignCluster); err != nil {
		return err
	}

	t.ClusterID = foreignCluster.Spec.ClusterIdentity.ClusterID

	// Update ForeignCluster
	foreignCluster.Spec.OutgoingPeeringEnabled = discoveryv1alpha1.PeeringEnabledNo
	return cl.Update(ctx, &foreignCluster)
}
