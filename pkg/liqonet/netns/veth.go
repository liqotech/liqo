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

package netns

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqoneterrors "github.com/liqotech/liqo/pkg/liqonet/errors"
	liqorouting "github.com/liqotech/liqo/pkg/liqonet/routing"
	"github.com/liqotech/liqo/pkg/liqonet/utils/links"
)

// CreateVethPair it will create veth pair in hostNetns and move one of them in gatewayNetns.
// hostNetns is the host netns and gatewayNetns is the gateway netns.
// Error is returned if something goes wrong.
func CreateVethPair(hostVethName, gatewayVethName string, hostNetns, gatewayNetns ns.NetNS,
	linkMTU int) (hostVeth, gatewayVeth net.Interface, err error) {
	if hostNetns == nil || gatewayNetns == nil {
		return hostVeth, gatewayVeth, &liqoneterrors.WrongParameter{
			Parameter: "hostNetns and gatewayNetns",
			Reason:    liqoneterrors.NotNil}
	}
	// Check if in hostNetns, aka host netns, exists an interface named as hostVethName.
	// If it exists than we remove it.
	if err := links.DeleteIFaceByName(hostVethName); err != nil {
		return hostVeth, gatewayVeth, fmt.Errorf("an error occurred while deleting interface {%s} in host network: %w",
			hostVethName, err)
	}

	var createVethPair = func() error {
		gatewayVeth, hostVeth, err = ip.SetupVethWithName(hostVethName, gatewayVethName, linkMTU, "", gatewayNetns)
		if err != nil {
			return fmt.Errorf("an error occurred while creating veth pair between host and gateway namespace: %w", err)
		}
		hostIface, err := netlink.LinkByName(hostVethName)
		if err != nil {
			return fmt.Errorf("an error occurred while getting interface {%s} in host netns with path {%s}: %w", hostVethName, hostNetns.Path(), err)
		}

		if err = netlink.LinkSetUp(hostIface); err != nil {
			return fmt.Errorf("an error occurred while setting UP interface {%s} in host netns with path {%s}: %w", hostVethName, hostNetns.Path(), err)
		}
		return nil
	}
	// If we just delete the old network namespace it would require some time for the kernel to
	// remove the veth device in the host network, so we retry in case of temporary conflicts.
	retryiable := func(err error) bool {
		return true
	}

	return hostVeth, gatewayVeth, retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 100 * time.Millisecond,
		Factor:   1.0,
		Jitter:   0.1,
	}, retryiable, createVethPair)
}

// ConfigureVeth configures the veth interface passed as argument. If the veth interface is the one
// living the gateway netns then additional actions are carried out.
func ConfigureVeth(veth *net.Interface, gatewayIP string, netNS ns.NetNS) error {
	var defaultCIDR = "0.0.0.0/0"

	gwIP := net.ParseIP(gatewayIP)
	if gwIP == nil {
		return &liqoneterrors.ParseIPError{
			IPToBeParsed: gatewayIP,
		}
	}

	gwNet := gatewayIP + "/32"
	klog.V(5).Infof("configuring veth {%s} with index {%d} in namespace with path {%s}",
		veth.Name, veth.Index, netNS.Path())

	configuration := func(netNamespace ns.NetNS) error {
		// Disable arp on the veth link.
		if err := netlink.LinkSetARPOff(&netlink.Veth{
			LinkAttrs: netlink.LinkAttrs{
				Index: veth.Index,
			},
		}); err != nil {
			return fmt.Errorf("unable to disable arp for veth {%s} with index {%d} in namespace {%s}: %w",
				veth.Name, veth.Index, netNS.Path(), err)
		}
		klog.V(5).Infof("arp correctly disabled for veth device {%s} with index {%d}", veth.Name, veth.Index)

		// Add route for the gatewayIP used as next hop for the default route.
		if _, err := liqorouting.AddRoute(gwNet, "", veth.Index, unix.RT_TABLE_MAIN, liqorouting.DefaultFlags, netlink.SCOPE_LINK); err != nil {
			return fmt.Errorf("unable to configure route for ip {%s} on device {%s} with index {%d}: %w",
				gatewayIP, veth.Name, veth.Index, err)
		}
		klog.V(5).Infof("route for ip {%s} correctly configured on device {%s} with index {%d}",
			gatewayIP, veth.Name, veth.Index)

		// The following configuration is done only for the veth pair living in the gateway network namespace.
		if veth.Name == liqoconst.GatewayVethName {
			// Add default route to use the veth interface.
			if _, err := liqorouting.AddRoute(defaultCIDR, gatewayIP, veth.Index, unix.RT_TABLE_MAIN,
				liqorouting.DefaultFlags, liqorouting.DefaultScope); err != nil {
				return fmt.Errorf("unable to configure route for ip {%s} on device {%s} with index {%d}: %w",
					defaultCIDR, veth.Name, veth.Index, err)
			}
			klog.V(5).Infof("route for ip {%s} correctly configured on device {%s} with index {%d}",
				defaultCIDR, veth.Name, veth.Index)

			// Enable ip forwarding in the gateway namespace.
			if err := liqorouting.EnableIPForwarding(); err != nil {
				return fmt.Errorf("unable to enable ip forwarding in namespace {%s}: %w", netNS.Path(), err)
			}
			klog.V(5).Infof("ipv4 forwarding in namespace {%s} with path {%s} correctly enabled",
				liqoconst.GatewayNetnsName, netNS.Path())
		}

		return nil
	}

	return netNS.Do(configuration)
}

// ConfigureVethNeigh configures an entry in the ARP table, according to the specified parameters.
func ConfigureVethNeigh(veth *net.Interface, gatewayIP string, gatewayMAC net.HardwareAddr, netNS ns.NetNS) error {
	gwIP := net.ParseIP(gatewayIP)
	if gwIP == nil {
		return &liqoneterrors.ParseIPError{IPToBeParsed: gatewayIP}
	}

	return netNS.Do(func(nn ns.NetNS) error {
		// Add static/permanent neighbor entry for the gateway IP address.
		if _, err := AddNeigh(gwIP, gatewayMAC, veth); err != nil {
			return fmt.Errorf("unable to add neighbor entry for ip {%s} and MAC {%s} on device {%s} with index {%d} in ns {%s}: %w",
				gatewayIP, gatewayMAC.String(), veth.Name, veth.Index, nn.Path(), err)
		}

		klog.V(5).Infof("neighbor entry for ip {%s} and MAC {%s} correctly configured on device {%s} with index {%d}",
			gatewayIP, gatewayMAC.String(), veth.Name, veth.Index)

		return nil
	})
}

// RegisterOnVethHwAddrChangeHandler registers a handler to be executed whenever an attribute of the given veth interface changes.
// The handler is always executed once upon registration.
func RegisterOnVethHwAddrChangeHandler(namespace ns.NetNS, vethName string, handler func(net.HardwareAddr) error) error {
	updates := make(chan netlink.LinkUpdate)

	if err := netlink.LinkSubscribeAt(netns.NsHandle(namespace.Fd()), updates, context.Background().Done()); err != nil {
		return err
	}

	// Immediately execute the handler, retrieving the updated value for the MAC address. This also ensures that the update is performed
	// once in the main thread, returning an appropriate error in case it fails.
	// Since we already subscribed to events, we can be sure that the handler will be executed again in case of further changes.
	veth, err := netlink.LinkByName(vethName)
	if err != nil {
		return fmt.Errorf("unable to retrieve veth interface {%s} in namespace {%s}: %w", vethName, namespace.Path(), err)
	}

	if err := handler(veth.Attrs().HardwareAddr); err != nil {
		return err
	}

	go func() {
		for {
			update := <-updates
			if update.Attrs().Name != vethName || (update.Attrs().Flags&net.FlagUp) == 0 {
				// Skip updates related to other interfaces, or in case it is down.
				continue
			}

			klog.V(5).Infof("received update for device {%s} with index {%d}, and hardware address {%s}",
				update.Attrs().Name, update.Attrs().Index, update.Attrs().HardwareAddr)

			if err := retry.OnError(retry.DefaultRetry, func(error) bool { return true },
				func() error { return handler(update.Attrs().HardwareAddr) }); err != nil {
				klog.Errorf("failed to handle MAC address change for veth {%s} with index {%d}: %v",
					update.Attrs().Name, update.Attrs().Index, err)
			}
		}
	}()

	return nil
}
