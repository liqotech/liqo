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

package main

import (
	"flag"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/liqotech/liqo/internal/liqonet/network-manager/netcfgcreator"
	"github.com/liqotech/liqo/internal/liqonet/network-manager/tunnelendpointcreator"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqonetIpam "github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

type networkManagerFlags struct {
	podCIDR     args.CIDR
	serviceCIDR args.CIDR

	additionalPools args.CIDRList
	reservedPools   args.CIDRList
}

func addNetworkManagerFlags(managerFlags *networkManagerFlags) {
	flag.Var(&managerFlags.podCIDR, "manager.pod-cidr", "The subnet used by the cluster for the pods, in CIDR notation")
	flag.Var(&managerFlags.serviceCIDR, "manager.service-cidr", "The subnet used by the cluster for the pods, in services notation")
	flag.Var(&managerFlags.reservedPools, "manager.reserved-pools",
		"Private CIDRs slices used by the Kubernetes infrastructure, in addition to the pod and service CIDR (e.g., the node subnet).")
	flag.Var(&managerFlags.additionalPools, "manager.additional-pools",
		"Network pools used to map a cluster network into another one in order to prevent conflicts, in addition to standard private CIDRs.")
}

func runNetworkManager(commonFlags *liqonetCommonFlags, managerFlags *networkManagerFlags) {
	podNamespace, err := utils.GetPodNamespace()
	if err != nil {
		klog.Errorf("unable to get pod namespace: %v", err)
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(restcfg.SetRateLimiter(ctrl.GetConfigOrDie()), ctrl.Options{
		MapperProvider:     mapper.LiqoMapperProvider(scheme),
		Scheme:             scheme,
		MetricsBindAddress: commonFlags.metricsAddr,
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: cache.SelectorsByObject{
				&corev1.Secret{}:  {Field: fields.OneTermEqualSelector("metadata.namespace", podNamespace)},
				&corev1.Service{}: {Field: fields.OneTermEqualSelector("metadata.namespace", podNamespace)},
			},
		}),
	})
	if err != nil {
		klog.Errorf("unable to get manager: %s", err)
		os.Exit(1)
	}
	dynClient := dynamic.NewForConfigOrDie(mgr.GetConfig())

	ipam, err := initializeIPAM(dynClient, managerFlags)
	if err != nil {
		klog.Errorf("Failed to initialize IPAM: %s", err)
		os.Exit(1)
	}

	externalCIDR, err := ipam.GetExternalCIDR(utils.GetMask(managerFlags.podCIDR.String()))
	if err != nil {
		klog.Errorf("Failed to initialize the external CIDR: %s", err)
		os.Exit(1)
	}

	tec := &tunnelendpointcreator.TunnelEndpointCreator{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		IPManager: ipam,
	}

	ncc := &netcfgcreator.NetworkConfigCreator{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		PodCIDR:      managerFlags.podCIDR.String(),
		ExternalCIDR: externalCIDR,
	}

	if err = tec.SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to create controller TunnelEndpointCreator: %s", err)
		os.Exit(1)
	}

	if err = ncc.SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to create controller NetworkConfigCreator: %s", err)
		os.Exit(1)
	}

	klog.Info("starting manager as liqo-network-manager")
	if err := mgr.Start(tec.SetupSignalHandlerForTunEndCreator()); err != nil {
		klog.Errorf("an error occurred while starting manager: %s", err)
		os.Exit(1)
	}
}

func initializeIPAM(client dynamic.Interface, managerFlags *networkManagerFlags) (*liqonetIpam.IPAM, error) {
	ipam := liqonetIpam.NewIPAM()

	if err := ipam.Init(liqonetIpam.Pools, client, liqoconst.NetworkManagerIpamPort); err != nil {
		return nil, err
	}

	if err := ipam.SetPodCIDR(managerFlags.podCIDR.String()); err != nil {
		return nil, err
	}
	if err := ipam.SetServiceCIDR(managerFlags.serviceCIDR.String()); err != nil {
		return nil, err
	}

	for _, pool := range managerFlags.additionalPools.StringList.StringList {
		if err := ipam.AddNetworkPool(pool); err != nil {
			return nil, err
		}
	}

	if err := ipam.SetReservedSubnets(managerFlags.reservedPools.StringList.StringList); err != nil {
		return nil, err
	}

	return ipam, nil
}
