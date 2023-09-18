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

package tunneloperator

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet/iptables"
	"github.com/liqotech/liqo/pkg/liqonet/netns"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

const (
	clusterName1 = "clusterName1"
	clusterName2 = "clusterName2"
	clusterID1   = "clusterID1" // clusterID1 string length must be less than 10.
	clusterID2   = "clusterID2" // clusterID2 string length must be less than 10.
	iptNetnsName = "iptNetNs"
)

var (
	ctx    context.Context
	cancel context.CancelFunc

	envTest            *envtest.Environment
	ipt                iptables.IPTHandler
	k8sClient          client.Client
	controller         *NatMappingController
	readyClustersMutex sync.Mutex
	readyClusters      = map[string]struct{}{clusterID1: {}}
	gatewayNetns       ns.NetNS
	iptNetns           ns.NetNS
	tep1               = &netv1alpha1.TunnelEndpoint{
		Spec: netv1alpha1.TunnelEndpointSpec{
			ClusterIdentity:       v1alpha1.ClusterIdentity{ClusterID: clusterID1, ClusterName: clusterName1},
			LocalPodCIDR:          "192.168.0.0/24",
			LocalNATPodCIDR:       "192.168.1.0/24",
			LocalExternalCIDR:     "192.168.3.0/24",
			LocalNATExternalCIDR:  "192.168.4.0/24",
			RemotePodCIDR:         "10.0.0.0/24",
			RemoteNATPodCIDR:      "10.0.70.0/24",
			RemoteExternalCIDR:    "10.0.1.0/24",
			RemoteNATExternalCIDR: "192.168.5.0/24",
		},
	}
	tep2 = &netv1alpha1.TunnelEndpoint{
		Spec: netv1alpha1.TunnelEndpointSpec{
			ClusterIdentity:       v1alpha1.ClusterIdentity{ClusterID: clusterID2, ClusterName: clusterName2},
			LocalPodCIDR:          "192.168.0.0/24",
			LocalNATPodCIDR:       "192.168.1.0/24",
			LocalExternalCIDR:     "192.168.3.0/24",
			LocalNATExternalCIDR:  "192.168.4.0/24",
			RemotePodCIDR:         "10.0.0.0/24",
			RemoteNATPodCIDR:      "10.0.70.0/24",
			RemoteExternalCIDR:    "10.0.1.0/24",
			RemoteNATExternalCIDR: "192.168.5.0/24",
		},
	}
	nm1     *netv1alpha1.NatMapping
	nm2     *netv1alpha1.NatMapping
	tc      = &TunnelController{}
	request = reconcile.Request{}
)

func TestTunnelOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TunnelOperator Suite")
}

var _ = BeforeSuite(func() {
	var err error

	testutil.LogsToGinkgoWriter()
	ctx, cancel = context.WithCancel(context.Background())

	// Create custom network namespace for tunnel-operator.
	gatewayNetns, err = netns.CreateNetns(consts.GatewayNetnsName)
	Expect(err).ShouldNot(HaveOccurred())
	tc.gatewayNetns = gatewayNetns
	tc.hostNetns, err = ns.GetCurrentNS()
	Expect(err).ShouldNot(HaveOccurred())

	// Create custom network namespace for natmapping-operator.
	iptNetns, err = netns.CreateNetns(iptNetnsName)
	Expect(err).ShouldNot(HaveOccurred())

	// Config IPTables for remote clusters
	err = initIPTables()
	Expect(err).ShouldNot(HaveOccurred())

	err = netv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).ShouldNot(HaveOccurred())
	envTest = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")},
	}
	config, err := envTest.Start()
	Expect(err).ShouldNot(HaveOccurred())
	mgr, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Scheme:  scheme.Scheme,
		Metrics: server.Options{BindAddress: "0"}, // this avoids port binding collision
	})
	Expect(err).ShouldNot(HaveOccurred())
	controller, err = NewNatMappingController(mgr.GetClient(), &readyClustersMutex, readyClusters, iptNetns)
	Expect(err).ShouldNot(HaveOccurred())
	go func() {
		if err = mgr.Start(ctx); err != nil {
			panic(err)
		}
	}()
	k8sClient = mgr.GetClient()
	// Create labeler test namespace.
	labNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: labelerNamespace,
		},
	}
	Eventually(func() error { return k8sClient.Create(context.TODO(), labNamespace) }).Should(BeNil())
	// We reconcile on a resource that does not exist with
	// an Eventually block in order to wait for
	// cache to start and then begin with unit tests.
	// If a resource does not exist, Reconcile returns
	// a nil error.
	request.Name = "wait-cache"
	request.Namespace = namespace
	Eventually(func() error {
		_, err := controller.Reconcile(context.Background(), request)
		return err
	}).Should(BeNil())
})

var _ = AfterSuite(func() {
	cancel()
	Expect(envTest.Stop()).To(Succeed())
	Expect(terminateIPTables()).To(Succeed())
	Expect(gatewayNetns.Close()).To(Succeed())
	Expect(iptNetns.Close()).To(Succeed())
})

func terminateIPTables() error {
	err := iptNetns.Do(func(nn ns.NetNS) error {
		var err error
		err = ipt.RemoveIPTablesConfigurationPerCluster(tep1)
		if err != nil {
			return err
		}
		err = ipt.RemoveIPTablesConfigurationPerCluster(tep2)
		if err != nil {
			return err
		}
		err = ipt.Terminate()
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

func initIPTables() error {
	err := iptNetns.Do(func(nn ns.NetNS) error {
		var err error
		// Allocate new IPTable handler
		ipt, err = iptables.NewIPTHandler()
		if err != nil {
			return err
		}
		if err := ipt.Init(); err != nil {
			return err
		}
		if err := ipt.EnsureChainsPerCluster(clusterID1); err != nil {
			return err
		}
		if err := ipt.EnsureChainsPerCluster(clusterID2); err != nil {
			return err
		}
		if err := ipt.EnsureChainRulesPerCluster(tep1); err != nil {
			return err
		}
		return ipt.EnsureChainRulesPerCluster(tep2)
	})
	return err
}
