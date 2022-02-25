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

package connect

import (
	"context"
	"fmt"
	"time"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/common"
)

// Args flags of the connect command.
type Args struct {
	Cluster1Namespace  string
	Cluster2Namespace  string
	Cluster1Kubeconfig string
	Cluster2Kubeconfig string
}

// Handler implements the logic of the connect command.
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

	// SetUp tenant namespace for cluster 2 in cluster 1.
	if err := cluster1.SetUpTenantNamespace(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}
	cluster2.SetRemTenantNS(cluster1.GetLocTenantNS())

	// SetUp tenant namespace for cluster  in cluster 2.
	if err := cluster2.SetUpTenantNamespace(ctx, cluster1.GetClusterID()); err != nil {
		return err
	}
	cluster1.SetRemTenantNS(cluster2.GetLocTenantNS())

	// Configure network configuration for cluster 1.
	if err := cluster1.ExchangeNetworkCfg(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Configure network configuration for cluster 2.
	if err := cluster2.ExchangeNetworkCfg(ctx, cluster1.GetClusterID()); err != nil {
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

	// Setting up proxy pod for cluster 1.
	if err := cluster1.SetUpProxy(ctx); err != nil {
		return err
	}

	// Setting up proxy pod for cluster 2.
	if err := cluster2.SetUpProxy(ctx); err != nil {
		return err
	}

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

	// Mapping proxy's ip in cluster1 for cluster2.
	if err := cluster1.MapProxyIPForCluster(ctx, ipamClient1, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Mapping proxy's ip in cluster2 for cluster2
	if err := cluster2.MapProxyIPForCluster(ctx, ipamClient2, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Mapping auth's ip  in cluster1 for cluster2.
	if err := cluster1.MapAuthIPForCluster(ctx, ipamClient1, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Mapping auth's ip in cluster2 for cluster1
	if err := cluster2.MapAuthIPForCluster(ctx, ipamClient2, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Creating foreign cluster in cluster1 for cluster 2.
	if err := cluster1.EnforceForeignCluster(ctx, cluster2.GetClusterID(), cluster2.GetAuthToken(),
		cluster2.GetAuthURL(), cluster2.GetProxyURL()); err != nil {
		return err
	}

	// Creating foreign cluster in cluster2 for cluster 1.
	if err := cluster2.EnforceForeignCluster(ctx, cluster1.GetClusterID(), cluster1.GetAuthToken(),
		cluster1.GetAuthURL(), cluster1.GetProxyURL()); err != nil {
		return err
	}

	// Waiting for authentication to complete in cluster 1.
	if err := cluster1.WaitForAuth(ctx, cluster2.GetClusterID(), 120*time.Second); err != nil {
		return err
	}

	// Waiting for authentication to complete in cluster 2.
	return cluster2.WaitForAuth(ctx, cluster1.GetClusterID(), 120*time.Second)
}
