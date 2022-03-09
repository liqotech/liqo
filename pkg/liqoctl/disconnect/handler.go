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

package disconnect

import (
	"context"
	"fmt"
	"time"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/common"
)

// Args flags of the disconnect command.
type Args struct {
	Cluster1Namespace  string
	Cluster2Namespace  string
	Cluster1Kubeconfig string
	Cluster2Kubeconfig string
}

// Handler implements the logic of the disconnect command.
func (a *Args) Handler(ctx context.Context) error {
	// Check that the kubeconfigs are different.
	if a.Cluster1Kubeconfig == a.Cluster2Kubeconfig {
		common.ErrorPrinter.Printf("kubeconfig1 and kubeconfig2 has to be different, current value: %s", a.Cluster2Kubeconfig)
		return fmt.Errorf("kubeconfig1 and kubeconfig2 has to be different, current value: %s", a.Cluster2Kubeconfig)
	}
	// Create restconfigs and clients for cluster 1.
	cl1RestCfg, err := clientcmd.BuildConfigFromFlags("", a.Cluster1Kubeconfig)
	if err != nil {
		common.ErrorPrinter.Printf("unable to create rest config from kubeconfig {%s}: %s", a.Cluster1Kubeconfig, err.Error())
		return err
	}
	cl1K8sClient, err := k8s.NewForConfig(cl1RestCfg)
	if err != nil {
		common.ErrorPrinter.Printf("unable to create client set from kubeconfig {%s}: %s", a.Cluster1Kubeconfig, err.Error())
		return err
	}
	cl1CRClient, err := client.New(cl1RestCfg, client.Options{
		Scheme: common.Scheme,
	})
	if err != nil {
		common.ErrorPrinter.Printf("unable to create controller runtime client from kubeconfig {%s}: %s",
			a.Cluster1Kubeconfig, err.Error())
		return err
	}

	// Create restconfigs and clients for cluster 2.
	cl2RestCfg, err := clientcmd.BuildConfigFromFlags("", a.Cluster2Kubeconfig)
	if err != nil {
		common.ErrorPrinter.Printf("unable to create rest config from kubeconfig {%s}: %s", a.Cluster2Kubeconfig, err.Error())
		return err
	}
	cl2K8sClient, err := k8s.NewForConfig(cl2RestCfg)
	if err != nil {
		common.ErrorPrinter.Printf("unable to create client set from kubeconfig {%s}: %s", a.Cluster2Kubeconfig, err.Error())
		return err
	}
	cl2CRClient, err := client.New(cl2RestCfg, client.Options{
		Scheme: common.Scheme,
	})
	if err != nil {
		common.ErrorPrinter.Printf("unable to create controller runtime client from kubeconfig {%s}: %s",
			a.Cluster2Kubeconfig, err.Error())
		return err
	}

	// Create and initialize cluster 1.
	cluster1 := common.NewCluster(cl1K8sClient, cl1CRClient, cl2CRClient, cl1RestCfg, a.Cluster1Namespace, common.Cluster1Name, common.Cluster1Color)
	if err := cluster1.Init(ctx); err != nil {
		return err
	}

	// Create and initialize cluster 2.
	cluster2 := common.NewCluster(cl2K8sClient, cl2CRClient, cl1CRClient, cl2RestCfg, a.Cluster2Namespace, common.Cluster2Name, common.Cluster2Color)
	if err := cluster2.Init(ctx); err != nil {
		return err
	}

	// Disable peering in cluster 1.
	if err := cluster1.DisablePeering(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Disable peering in cluster 2.
	if err := cluster2.DisablePeering(ctx, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Wait to unpeer in cluster 1.
	if err := cluster1.WaitForUnpeering(ctx, cluster2.GetClusterID(), 60*time.Second); err != nil {
		return err
	}

	// Disable peering in cluster 2.
	if err := cluster2.WaitForUnpeering(ctx, cluster1.GetClusterID(), 60*time.Second); err != nil {
		return err
	}

	// Delete foreigncluster of cluster2 in cluster1.
	if err := cluster1.DeleteForeignCluster(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Delete foreigncluster of cluster1 in cluster2.
	if err := cluster2.DeleteForeignCluster(ctx, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Clean up tenant namespace in cluster 1.
	if err := cluster1.TearDownTenantNamespace(ctx, cluster2.GetClusterID(), 60*time.Second); err != nil {
		return err
	}

	// Clean up tenant namespace in cluster 2.
	if err := cluster2.TearDownTenantNamespace(ctx, cluster1.GetClusterID(), 60*time.Second); err != nil {
		return err
	}

	// Port-forwarding ipam service for cluster 1.
	if err := cluster1.PortForwardIPAM(ctx); err != nil {
		return err
	}
	defer cluster1.StopPortForwardIPAM()

	// Port-forwarding ipam service for cluster 2.
	if err := cluster2.PortForwardIPAM(ctx); err != nil {
		return err
	}
	defer cluster2.StopPortForwardIPAM()

	// Creating IPAM client for cluster 1.
	ipamClient1, err := cluster1.NewIPAMClient(ctx)
	if err != nil {
		return err
	}

	// Creating IPAM client for cluster 2.
	ipamClient2, err := cluster2.NewIPAMClient(ctx)
	if err != nil {
		return err
	}

	// Unmapping proxy's ip in cluster1 for cluster2.
	if err := cluster1.UnmapProxyIPForCluster(ctx, ipamClient1, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Unmapping proxy's ip in cluster2 for cluster2
	if err := cluster2.UnmapProxyIPForCluster(ctx, ipamClient2, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Unmapping auth's ip  in cluster1 for cluster2.
	if err := cluster1.UnmapAuthIPForCluster(ctx, ipamClient1, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Unapping auth's ip in cluster2 for cluster1
	return cluster2.UnmapAuthIPForCluster(ctx, ipamClient2, cluster1.GetClusterID())
}
