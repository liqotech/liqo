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

package integration_tests_test

import (
	"context"
	"crypto/rand"
	"math/big"
	"path/filepath"
	sync "sync"
	"testing"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	tunneloperator "github.com/liqotech/liqo/internal/liqonet/tunnel-operator"
	liqonetIpam "github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/liqonet/iptables"
	"github.com/liqotech/liqo/pkg/liqonet/netns"
)

const (
	iptNetnsName       = "iptNetNs"
	clusterID1         = "cluster1"
	clusterID2         = "cluster2"
	remotePodCIDR      = "10.50.0.0/16"
	remoteExternalCIDR = "10.60.0.0/16"
	localPodCIDR       = "10.70.0.0/16"
	localExternalCIDR  = "10.80.0.0/16"
	homePodCIDR        = "10.0.0.0/24"
	remoteEndpointIP   = "12.0.3.4"
	remoteEndpointIP2  = "12.0.5.4"
	timeout            = time.Second * 10
	interval           = time.Millisecond * 250
)

var (
	err                error
	envTest            *envtest.Environment
	ipt                iptables.IPTHandler
	ipam               *liqonetIpam.IPAM
	homeExternalCIDR   string
	k8sClient          client.Client
	dynClient          dynamic.Interface
	controller         *tunneloperator.NatMappingController
	readyClustersMutex sync.Mutex
	readyClusters      = map[string]struct{}{
		clusterID1: {},
		clusterID2: {},
	}
	iptNetns ns.NetNS
	tep1     = &netv1alpha1.TunnelEndpoint{
		Spec: netv1alpha1.TunnelEndpointSpec{
			ClusterID:             clusterID1,
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
			ClusterID:             clusterID2,
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
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var _ = BeforeSuite(func() {
	// Create custom network namespace for natmapping-operator.
	iptNetns, err = netns.CreateNetns(iptNetnsName)
	Expect(err).To(BeNil())

	// Config NAT driver for remote clusters
	err = initNATDriver()
	Expect(err).To(BeNil())

	// Launch natmapping operator
	err = initNatMappingController()
	Expect(err).To(BeNil())

	// Init Ipam
	err = initIpam()
	Expect(err).To(BeNil())
})

var _ = AfterSuite(func() {
	err := envTest.Stop()
	Expect(err).To(BeNil())
	err = terminateNATDriver()
	Expect(err).To(BeNil())
	err = iptNetns.Close()
	Expect(err).To(BeNil())
})

func initNatMappingController() error {
	err = netv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	envTest = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "deployments", "liqo", "crds")},
	}
	config, err := envTest.Start()
	if err != nil {
		return err
	}
	mgr, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})

	controller, err = tunneloperator.NewNatMappingController(mgr.GetClient(), &readyClustersMutex, readyClusters, iptNetns)
	if err != nil {
		return err
	}
	go func() {
		if err = mgr.Start(context.Background()); err != nil {
			panic(err)
		}
	}()
	dynClient = dynamic.NewForConfigOrDie(mgr.GetConfig())
	k8sClient = mgr.GetClient()
	return controller.SetupWithManager(mgr)
}

func initIpam() error {
	ipam = liqonetIpam.NewIPAM()
	n, err := rand.Int(rand.Reader, big.NewInt(2000))
	if err != nil {
		return err
	}
	err = ipam.Init(liqonetIpam.Pools, dynClient, 2000+int(n.Int64()))
	if err != nil {
		return err
	}
	// Set home cluster PodCIDR and ExternalCIDR
	err = ipam.SetPodCIDR(homePodCIDR)
	if err != nil {
		return err
	}
	homeExternalCIDR, err = ipam.GetExternalCIDR(uint8(24))
	if err != nil {
		return err
	}
	// Assign networks to clusterID1
	_, _, err = ipam.GetSubnetsPerCluster(remotePodCIDR, remoteExternalCIDR, clusterID1)
	if err != nil {
		return err
	}
	err = ipam.AddLocalSubnetsPerCluster(localPodCIDR, localExternalCIDR, clusterID1)
	if err != nil {
		return err
	}
	// Assign networks to clusterID2
	_, _, err = ipam.GetSubnetsPerCluster(remotePodCIDR, remoteExternalCIDR, clusterID2)
	if err != nil {
		return err
	}
	return ipam.AddLocalSubnetsPerCluster(localPodCIDR, localExternalCIDR, clusterID2)
}

func terminateNATDriver() error {
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

func initNATDriver() error {
	err := iptNetns.Do(func(nn ns.NetNS) error {
		var err error
		// Allocate new IPTables handler
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
		if err := ipt.EnsureChainRulesPerCluster(tep1); err != nil {
			return err
		}
		if err := ipt.EnsureChainsPerCluster(clusterID2); err != nil {
			return err
		}
		if err := ipt.EnsureChainRulesPerCluster(tep2); err != nil {
			return err
		}
		return nil
	})
	return err
}
