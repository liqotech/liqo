// Copyright 2019-2024 The Liqo Authors
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

package netmonitor

import (
	"context"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// Options defines the options for the interfaces monitoring.
// If the option is true, the monitoring will be enabled for that type of change.
// Possible options are: link, address, route.
type Options struct {
	Link  bool
	Addr  bool
	Route bool
}

// InterfacesMonitoring starts the monitoring of the network interfaces.
// If there is a change in the network interfaces, it will send a message to the channel.
// With the options, you can choose to monitor only the link, address, or route changes (default: all options are true).
func InterfacesMonitoring(ctx context.Context, eventChannel chan event.GenericEvent, options *Options) error {
	// Create channels to receive notifications for link, address, and route changes
	chLink := make(chan netlink.LinkUpdate)
	chAddr := make(chan netlink.AddrUpdate)
	chRoute := make(chan netlink.RouteUpdate)

	// Create maps to keep track of interfaces
	interfaces := make(map[string]bool)

	// If options are not specified, set the default options
	if options == nil {
		options = &Options{
			Link:  true,
			Addr:  true,
			Route: true,
		}
	}

	if options.Link {
		// Subscribe to the link updates
		if err := netlink.LinkSubscribe(chLink, ctx.Done()); err != nil {
			klog.Error(err)
			return err
		}

		// Get the list of existing links and add them to the interfaces map
		links, err := netlink.LinkList()
		if err != nil {
			klog.Error(err)
			return err
		}
		for _, link := range links {
			interfaces[link.Attrs().Name] = true
		}
	}

	if options.Addr {
		// Subscribe to the address updates
		if err := netlink.AddrSubscribe(chAddr, ctx.Done()); err != nil {
			klog.Error(err)
			return err
		}
	}

	if options.Route {
		// Subscribe to the route updates
		if err := netlink.RouteSubscribe(chRoute, ctx.Done()); err != nil {
			klog.Error(err)
			return err
		}
	}

	// Start an infinite loop to handle the notifications
	for {
		select {
		case updateLink := <-chLink:
			if options.Link {
				handleLinkUpdate(&updateLink, interfaces, eventChannel)
			}
		case updateAddr := <-chAddr:
			if options.Addr {
				handleAddrUpdate(&updateAddr, eventChannel)
			}
		case updateRoute := <-chRoute:
			if options.Route {
				handleRouteUpdate(&updateRoute, eventChannel)
			}
		case <-ctx.Done():
			klog.Info("Stop monitoring network interfaces.")
			return nil
		}
	}
}

func handleLinkUpdate(updateLink *netlink.LinkUpdate, interfaces map[string]bool, eventChannel chan<- event.GenericEvent) {
	switch {
	case updateLink.Header.Type == syscall.RTM_DELLINK:
		// Link has been removed
		klog.Infof("Interface removed: %s", updateLink.Link.Attrs().Name)
		delete(interfaces, updateLink.Link.Attrs().Name)
	case !interfaces[updateLink.Link.Attrs().Name] && updateLink.Header.Type == syscall.RTM_NEWLINK:
		// New link has been added
		klog.Infof("Interface added: %s", updateLink.Link.Attrs().Name)
		interfaces[updateLink.Link.Attrs().Name] = true
	case updateLink.Header.Type == syscall.RTM_NEWLINK:
		// Link has been modified
		if updateLink.Link.Attrs().Flags&net.FlagUp != 0 {
			klog.Infof("Interface %s is up", updateLink.Link.Attrs().Name)
		} else {
			klog.Infof("Interface %s is down", updateLink.Link.Attrs().Name)
		}
	default:
		klog.Warning("Unknown link update type.")
	}
	send(eventChannel)
}

func handleAddrUpdate(updateAddr *netlink.AddrUpdate, eventChannel chan<- event.GenericEvent) {
	iface, err := net.InterfaceByIndex(updateAddr.LinkIndex)
	if err != nil {
		// This case is not a real error, it happens when an up interface is removed, so the address is removed too,
		// so there is no need to call the reconcile since is already called by the interface update.
		klog.Infof("Address (%s) removed from the deleted interface", updateAddr.LinkAddress.IP)
		return
	}
	if updateAddr.NewAddr {
		// New address has been added
		klog.Infof("New address (%s) added to the interface: %s", updateAddr.LinkAddress.IP, iface.Name)
	} else {
		// Address has been removed
		klog.Infof("Address (%s) removed from the interface: %s", updateAddr.LinkAddress.IP, iface.Name)
	}
	send(eventChannel)
}

func handleRouteUpdate(updateRoute *netlink.RouteUpdate, eventChannel chan<- event.GenericEvent) {
	if updateRoute.Type == syscall.RTM_NEWROUTE {
		// New route has been added
		klog.Infof("New route added: %s", updateRoute.Route.Dst)
	} else if updateRoute.Type == syscall.RTM_DELROUTE {
		// Route has been removed
		klog.Infof("Route removed: %s", updateRoute.Route.Dst)
	}
	send(eventChannel)
}

// Send a channel with generic event type.
func send(eventChannel chan<- event.GenericEvent) {
	// Triggers a new reconcile
	ge := event.GenericEvent{}
	eventChannel <- ge
}
