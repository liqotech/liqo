// Copyright 2019-2025 The Liqo Authors
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
	"fmt"
	"net"
	"syscall"

	"github.com/google/nftables"
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// OptionsLink defines the options for the links monitoring.
type OptionsLink struct {
	Create bool
	Delete bool
	Modify bool
}

// OptionsAddr defines the options for the addresses monitoring.
type OptionsAddr struct {
	Create bool
	Delete bool
}

// OptionsRoute defines the options for the routes monitoring.
type OptionsRoute struct {
	Create bool
	Delete bool
}

// OptionsNftables defines the options for the nftables monitoring.
type OptionsNftables struct {
	Create bool
	Delete bool
}

// Options defines the options for the network monitoring.
type Options struct {
	Link     *OptionsLink
	Addr     *OptionsAddr
	Route    *OptionsRoute
	Nftables *OptionsNftables
}

// InterfacesMonitoring starts the monitoring of the network interfaces.
// If there is a change in the network interfaces, it will send a message to the channel.
// With the options, you can choose to monitor only the link, address, or route changes (default: all options are true).
func InterfacesMonitoring(ctx context.Context, eventChannel chan event.GenericEvent, options *Options) error {
	// Create channels to receive notifications for link, address, and route changes
	chLink := make(chan netlink.LinkUpdate)
	chAddr := make(chan netlink.AddrUpdate)
	chRoute := make(chan netlink.RouteUpdate)
	chNft := make(chan *nftables.MonitorEvent)

	// Create maps to keep track of interfaces
	interfaces := make(map[string]bool)

	// If options are not specified, set the default options
	if options == nil {
		return fmt.Errorf("options not specified")
	}

	if options.Link != nil {
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

	if options.Addr != nil {
		// Subscribe to the address updates
		if err := netlink.AddrSubscribe(chAddr, ctx.Done()); err != nil {
			klog.Error(err)
			return err
		}
	}

	if options.Route != nil {
		// Subscribe to the route updates
		if err := netlink.RouteSubscribe(chRoute, ctx.Done()); err != nil {
			klog.Error(err)
			return err
		}
	}

	if options.Nftables != nil {
		// Subscribe to the nftables updates
		conn, err := nftables.New()
		if err != nil {
			klog.Error(err)
			return err
		}
		mon := nftables.NewMonitor()
		defer mon.Close()
		chNft, err = conn.AddMonitor(mon)
		if err != nil {
			klog.Error(err)
			return err
		}
	}

	// Start an infinite loop to handle the notifications
	for {
		select {
		case updateLink := <-chLink:
			klog.V(4).Info("Link update received")
			if options.Link != nil {
				handleLinkUpdate(&updateLink, options.Link, interfaces, eventChannel)
			}
		case updateAddr := <-chAddr:
			klog.V(4).Info("Addr update received")
			if options.Addr != nil {
				handleAddrUpdate(&updateAddr, options.Addr, eventChannel)
			}
		case updateRoute := <-chRoute:
			klog.V(4).Info("Route update received")
			if options.Route != nil {
				handleRouteUpdate(&updateRoute, options.Route, eventChannel)
			}
		case updateNft := <-chNft:
			klog.V(4).Info("Nft update received")
			if updateNft != nil && options.Nftables != nil {
				handleNftUpdate(updateNft, options.Nftables, eventChannel)
			}
		case <-ctx.Done():
			klog.Info("Stop monitoring network interfaces.")
			return nil
		}
	}
}

func handleLinkUpdate(updateLink *netlink.LinkUpdate, optionsLink *OptionsLink, interfaces map[string]bool, eventChannel chan<- event.GenericEvent) {
	canSend := false
	switch {
	case updateLink.Header.Type == syscall.RTM_DELLINK:
		if optionsLink.Delete {
			canSend = true
			// Link has been removed
			klog.Infof("Interface removed: %s", updateLink.Link.Attrs().Name)
		}
		delete(interfaces, updateLink.Link.Attrs().Name)
	case !interfaces[updateLink.Link.Attrs().Name] && updateLink.Header.Type == syscall.RTM_NEWLINK:
		if optionsLink.Create {
			canSend = true
			// New link has been added
			klog.Infof("Interface added: %s", updateLink.Link.Attrs().Name)
			interfaces[updateLink.Link.Attrs().Name] = true
		}
	case updateLink.Header.Type == syscall.RTM_NEWLINK:
		// Link has been modified
		if optionsLink.Modify {
			canSend = true
			if updateLink.Link.Attrs().Flags&net.FlagUp != 0 {
				klog.Infof("Interface %s is up", updateLink.Link.Attrs().Name)
			} else {
				klog.Infof("Interface %s is down", updateLink.Link.Attrs().Name)
			}
		}
	default:
		klog.Warning("Unknown link update type.")
	}
	if canSend {
		send(eventChannel)
	}
}

func handleAddrUpdate(updateAddr *netlink.AddrUpdate, optionsAddr *OptionsAddr, eventChannel chan<- event.GenericEvent) {
	canSend := false
	iface, err := net.InterfaceByIndex(updateAddr.LinkIndex)
	if err != nil {
		// This case is not a real error, it happens when an up interface is removed, so the address is removed too,
		// so there is no need to call the reconcile since is already called by the interface update.
		klog.Infof("Address (%s) removed from the deleted interface", updateAddr.LinkAddress.IP)
		return
	}
	if updateAddr.NewAddr {
		if optionsAddr.Create {
			canSend = true
			// New address has been added
			klog.Infof("New address (%s) added to the interface: %s", updateAddr.LinkAddress.IP, iface.Name)
		}
	} else {
		if optionsAddr.Delete {
			canSend = true
			// Address has been removed
			klog.Infof("Address (%s) removed from the interface: %s", updateAddr.LinkAddress.IP, iface.Name)
		}
	}

	if canSend {
		send(eventChannel)
	}
}

func handleRouteUpdate(updateRoute *netlink.RouteUpdate, optionsRoute *OptionsRoute, eventChannel chan<- event.GenericEvent) {
	canSend := false
	if updateRoute.Type == syscall.RTM_NEWROUTE {
		if optionsRoute.Create {
			canSend = true
			// New route has been added
			klog.Infof("New route added: %s", updateRoute.Route.Dst)
		}
	} else if updateRoute.Type == syscall.RTM_DELROUTE {
		if optionsRoute.Delete {
			canSend = true
			// Route has been removed
			klog.Infof("Route removed: %s", updateRoute.Route.Dst)
		}
	}
	if canSend {
		send(eventChannel)
	}
}

func handleNftUpdate(updateNft *nftables.MonitorEvent, optionsNftables *OptionsNftables, eventChannel chan<- event.GenericEvent) {
	canSend := false
	switch updateNft.Type {
	case nftables.MonitorEventTypeDelTable:
		if optionsNftables.Delete {
			name := updateNft.Data.(*nftables.Table).Name
			if name != "" {
				// Nftables table has been removed
				klog.Infof("Nftables table removed: %s", name)
				canSend = true
			}
		}
	case nftables.MonitorEventTypeDelChain:
		if optionsNftables.Delete {
			name := updateNft.Data.(*nftables.Chain).Name
			if name != "" {
				// Nftables chain has been removed
				klog.Infof("Nftables chain removed: %s", name)
				canSend = true
			}
		}
	case nftables.MonitorEventTypeDelRule:
		if optionsNftables.Delete {
			name := updateNft.Data.(*nftables.Rule).UserData
			if len(name) != 0 {
				// Nftables rule has been removed
				klog.Infof("Nftables rule removed: %s", name)
				canSend = true
			}
		}
	default:
	}
	if canSend {
		send(eventChannel)
	}
}

// Send a channel with generic event type.
func send(eventChannel chan<- event.GenericEvent) {
	// Triggers a new reconcile
	ge := event.GenericEvent{}
	eventChannel <- ge
}
