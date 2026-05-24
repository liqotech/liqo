// Copyright 2019-2026 The Liqo Authors
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

package wireguard

import (
	"context"
	"errors"
	"fmt"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
)

// InitWireguardLink inits the Wireguard interface.
func InitWireguardLink(ctx context.Context, options *Options, idx int, port int) error {
	name := tunnel.GetTunnelName(idx)
	exists, err := existsLink(idx)
	if err != nil {
		return fmt.Errorf("cannot check if Wireguard interface %q exists: %w", name, err)
	}
	if exists {
		klog.Infof("Wireguard interface %q already exists", name)
		return nil
	}

	if err := createLink(ctx, options, idx, port); err != nil {
		return fmt.Errorf("cannot create Wireguard interface %q: %w", name, err)
	}

	link, err := tunnel.GetLink(name)
	if err != nil {
		return fmt.Errorf("cannot get Wireguard interface %q: %w", name, err)
	}

	klog.Infof("Setting up Wireguard interface %q with IP %q", name, tunnel.GetInterfaceIP(options.GwOptions.Mode, idx))
	if err := tunnel.AddAddress(link, tunnel.GetInterfaceIP(options.GwOptions.Mode, idx)); err != nil {
		return err
	}

	return netlink.LinkSetUp(link)
}

// createLink creates a new Wireguard interface.
func createLink(ctx context.Context, options *Options, idx int, port int) error {
	var err error
	klog.Infof("Selected wireguard %s implementation", options.Implementation)

	switch options.Implementation {
	case WgImplementationKernel:
		err = createLinkKernel(options, idx)
	case WgImplementationUserspace:
		err = createLinkUserspace(ctx, options, idx)
	default:
		err = fmt.Errorf("invalid wireguard implementation: %s", options.Implementation)
	}

	if err != nil {
		return fmt.Errorf("cannot create Wireguard interface %q: %w", tunnel.GetTunnelName(idx), err)
	}

	if options.GwOptions.Mode == gateway.ModeServer {
		wgcl, err := wgctrl.New()
		if err != nil {
			return fmt.Errorf("cannot create Wireguard client (interface %q): %w", tunnel.GetTunnelName(idx), err)
		}
		defer wgcl.Close()

		if err := wgcl.ConfigureDevice(tunnel.GetTunnelName(idx), wgtypes.Config{
			ListenPort: &port,
		}); err != nil {
			return fmt.Errorf("cannot configure Wireguard interface %q: %w", tunnel.GetTunnelName(idx), err)
		}
	}

	return nil
}

// createLinkKernel creates a new Wireguard interface using the kernel module.
func createLinkKernel(options *Options, idx int) error {
	link := netlink.Wireguard{
		LinkAttrs: netlink.LinkAttrs{
			MTU:  options.MTU,
			Name: tunnel.GetTunnelName(idx),
		},
	}

	err := netlink.LinkAdd(&link)
	if err != nil {
		return fmt.Errorf("cannot add Wireguard interface %q: %w", tunnel.GetTunnelName(idx), err)
	}
	return nil
}

// createLinkUserspace creates a new Wireguard interface using the userspace implementation (wireguard-go)
// embedded as a library. The device is kept running in-process for the lifetime of the gateway.
func createLinkUserspace(ctx context.Context, options *Options, idx int) error {
	mtu := options.MTU
	if mtu <= 0 {
		mtu = device.DefaultMTU
	}

	tunDev, err := tun.CreateTUN(tunnel.GetTunnelName(idx), mtu)
	if err != nil {
		return fmt.Errorf("failed to create wireguard TUN device %q: %w", tunnel.GetTunnelName(idx), err)
	}

	name, err := tunDev.Name()
	if err != nil {
		_ = tunDev.Close()
		return fmt.Errorf("failed to read wireguard TUN device name: %w", err)
	}

	fileUAPI, err := ipc.UAPIOpen(name)
	if err != nil {
		_ = tunDev.Close()
		return fmt.Errorf("failed to open UAPI socket for %q: %w", name, err)
	}

	logger := device.NewLogger(device.LogLevelError, fmt.Sprintf("(%s) ", name))
	wgDev := device.NewDevice(tunDev, conn.NewDefaultBind(), logger)

	uapi, err := ipc.UAPIListen(name, fileUAPI)
	if err != nil {
		wgDev.Close()
		return fmt.Errorf("failed to listen on UAPI socket for %q: %w", name, err)
	}

	go func() {
		for {
			c, err := uapi.Accept()
			if err != nil {
				return
			}
			go wgDev.IpcHandle(c)
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			if err := uapi.Close(); err != nil {
				klog.Errorf("Error closing uapi: %v", err)
			}
			wgDev.Close()
		case <-wgDev.Wait():
			if err := uapi.Close(); err != nil {
				klog.Errorf("Error closing uapi: %v", err)
			}
			klog.Fatalf("wireguard userspace device %q stopped unexpectedly", name)
		}
	}()

	klog.Infof("wireguard userspace device %q started", name)
	return nil
}

func existsLink(idx int) (bool, error) {
	_, err := tunnel.GetLink(tunnel.GetTunnelName(idx))
	if err != nil {
		if errors.As(err, &netlink.LinkNotFoundError{}) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetWireguardPorts returns the list of ports to be used for WireGuard interfaces.
func GetWireguardPorts(opts *Options) []int {
	var ports []int

	switch opts.GwOptions.Mode {
	case gateway.ModeClient:
		if len(opts.EndpointPorts) > 0 {
			ports = opts.EndpointPorts
		} else if opts.EndpointPort != 0 {
			ports = []int{opts.EndpointPort}
		}

	case gateway.ModeServer:
		if len(opts.ListenPorts) > 0 {
			ports = opts.ListenPorts
		} else if opts.ListenPort != 0 {
			ports = []int{opts.ListenPort}
		}
	}

	if len(ports) > tunnel.MaxWireguardInterfaces {
		klog.Warningf("Requested %d WireGuard interfaces, capping to maximum of %d", len(ports), tunnel.MaxWireguardInterfaces)
		ports = ports[:tunnel.MaxWireguardInterfaces]
	}

	return ports
}
