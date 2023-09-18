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

package main

import (
	"context"
	"flag"
	"os"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	routeoperator "github.com/liqotech/liqo/internal/liqonet/route-operator"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	liqorouting "github.com/liqotech/liqo/pkg/liqonet/routing"
	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
	liqonetsignals "github.com/liqotech/liqo/pkg/liqonet/utils/signals"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

type routeOperatorFlags struct {
	vni      int
	mtu      int
	vtepPort int
}

func addRouteOperatorFlags(liqonet *routeOperatorFlags) {
	flag.IntVar(&liqonet.vni, "route.vxlan-vni", 18952, "VXLAN Virtual Network Identifier (VNI) for the Liqonet intra-cluster overlay network")
	flag.IntVar(&liqonet.mtu, "route.vxlan-mtu", liqoconst.DefaultMTU, "VXLAN Max Transmit Unit (MTU) for the Liqonet intra-cluster overlay network")
	flag.IntVar(&liqonet.vtepPort, "route.vxlan-vtep-port", 4879,
		"VXLAN Virtual Tunnel Endpoints (VTEP) port for the Liqonet intra-cluster overlay network")
}

func runRouteOperator(commonFlags *liqonetCommonFlags, routeFlags *routeOperatorFlags) {
	ctx, _ := liqonetsignals.NotifyContextPosix(context.Background(), liqonetsignals.ShutdownSignals...)
	vxlanConfig := &overlay.VxlanDeviceAttrs{
		Vni:      routeFlags.vni,
		Name:     liqoconst.VxlanDeviceName,
		VtepPort: routeFlags.vtepPort,
		VtepAddr: nil,
		MTU:      routeFlags.mtu,
	}

	mutex := &sync.RWMutex{}
	nodeMap := map[string]string{}
	// Get the pod ip and parse to net.IP.
	podIP, err := liqonetutils.GetPodIP()
	if err != nil {
		klog.Errorf("unable to get podIP: %v", err)
		os.Exit(1)
	}
	podName, err := liqonetutils.GetPodName()
	if err != nil {
		klog.Errorf("unable to get pod name: %v", err)
		os.Exit(1)
	}
	nodeName, err := liqonetutils.GetNodeName()
	if err != nil {
		klog.Errorf("unable to get node name: %v", err)
		os.Exit(1)
	}
	podNamespace, err := liqonetutils.GetPodNamespace()
	if err != nil {
		klog.Errorf("unable to get pod namespace: %v", err)
		os.Exit(1)
	}
	// Asking the api-server to only inform the operator for the pods running in a node different from the one
	// where the operator is running, and that have a valid podIP set. The latter check also prevents newly created
	// pods scheduled on a virtual node to be temporarily considered while waiting for the appropriate label to be
	// added by the virtual kubelet. Indeed, when pods are created the label is not present, but we are sure that
	// it will be added before the IP address for the same pod is set.
	smcFieldSelector := fields.AndSelectors(
		fields.OneTermNotEqualSelector("spec.nodeName", nodeName),
		fields.OneTermNotEqualSelector("status.podIP", ""),
	)
	// Asking the api-server to only inform the operator for the pods running in a node different from
	// the virtual nodes. We want to process only the pods running on the local cluster and not the ones
	// offloaded to a remote cluster.
	smcLabelRequirement, err := labels.NewRequirement(liqoconst.LocalPodLabelKey, selection.NotEquals, []string{liqoconst.LocalPodLabelValue})
	utilruntime.Must(err)

	smcLabelSelector := labels.NewSelector().Add(*smcLabelRequirement)
	mainMgr, err := ctrl.NewManager(restcfg.SetRateLimiter(ctrl.GetConfigOrDie()), ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Metrics: server.Options{
			BindAddress: commonFlags.metricsAddr,
		},
		NewCache: func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
			opts.ByObject = map[client.Object]cache.ByObject{
				&corev1.Pod{}: {
					Field: smcFieldSelector,
					Label: smcLabelSelector,
				},
			}
			return cache.New(config, opts)
		},
	})
	if err != nil {
		klog.Errorf("unable to get manager: %s", err)
		os.Exit(1)
	}
	// Asking the api-server to only inform the operator for the pods that are part of the route component.
	ovcLabelSelector := labels.SelectorFromSet(labels.Set{
		podNameLabelKey:     routeNameLabelValue,
		podInstanceLabelKey: routeInstanceLabelValue,
	})
	// This manager is used by the overlay operator and it is limited to the pods running
	// on the same namespace as the operator.
	overlayMgr, err := ctrl.NewManager(mainMgr.GetConfig(), ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Metrics:        server.Options{BindAddress: "0"}, // Disable the metrics of the auxiliary manager to prevent conflicts.
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				podNamespace: {},
			},
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Pod{}: {
					Label: ovcLabelSelector,
				},
			},
		},
	})
	if err != nil {
		klog.Errorf("unable to get manager: %s", err)
		os.Exit(1)
	}
	vxlanConfig.VtepAddr = podIP
	if vxlanConfig.MAC, err = overlay.GenerateVxlanMac(nodeName); err != nil {
		klog.Errorf("unable to generate vxlan mac: %s", err)
		os.Exit(1)
	}
	klog.Infof("vxlan mac address: %s", vxlanConfig.MAC.String())
	vxlanDevice, err := overlay.NewVxlanDevice(vxlanConfig)
	if err != nil {
		klog.Errorf("an error occurred while creating vxlan device : %v", err)
		os.Exit(1)
	}
	go overlay.CheckVxlanDevice(ctx, vxlanDevice.Link)

	vxlanRoutingManager, err := liqorouting.NewVxlanRoutingManager(liqoconst.RoutingTableID,
		podIP.String(), liqoconst.OverlayNetPrefix, vxlanDevice)
	if err != nil {
		klog.Errorf("an error occurred while creating the vxlan routing manager: %v", err)
		os.Exit(1)
	}
	eventRecorder := mainMgr.GetEventRecorderFor(liqoconst.LiqoRouteOperatorName + "." + podIP.String())
	routeController := routeoperator.NewRouteController(podIP.String(), vxlanDevice, vxlanRoutingManager, eventRecorder, mainMgr.GetClient())
	if err = routeController.SetupWithManager(mainMgr); err != nil {
		klog.Errorf("unable to setup controller: %s", err)
		os.Exit(1)
	}
	if err = routeController.ConfigureFirewall(); err != nil {
		klog.Errorf("unable to start go routine that configures firewall rules for the route controller: %v", err)
		os.Exit(1)
	}
	overlayController, err := routeoperator.NewOverlayController(podName, vxlanDevice, mutex, nodeMap, overlayMgr.GetClient())
	if err != nil {
		klog.Errorf("an error occurred while creating overlay controller: %v", err)
		os.Exit(3)
	}
	if err = overlayController.SetupWithManager(overlayMgr); err != nil {
		klog.Errorf("unable to setup overlay controller: %s", err)
		os.Exit(1)
	}
	symmetricRoutingController, err := routeoperator.NewSymmetricRoutingOperator(nodeName,
		liqoconst.RoutingTableID, vxlanDevice, mutex, nodeMap, mainMgr.GetClient())
	if err != nil {
		klog.Errorf("an error occurred while creting symmetric routing controller: %v", err)
		os.Exit(4)
	}
	if err = symmetricRoutingController.SetupWithManager(mainMgr); err != nil {
		klog.Errorf("unable to setup overlay controller: %s", err)
		os.Exit(1)
	}
	if err := mainMgr.Add(overlayMgr); err != nil {
		klog.Errorf("unable to add the overlay manager to the main manager: %s", err)
		os.Exit(1)
	}
	if err := mainMgr.Start(routeController.SetupSignalHandlerForRouteOperator(ctx)); err != nil {
		klog.Errorf("unable to start controller: %s", err)
		os.Exit(1)
	}
}
