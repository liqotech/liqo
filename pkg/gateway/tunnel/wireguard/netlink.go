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
func InitWireguardLink(ctx context.Context, options *Options) error {
	exists, err := existsLink()
	if err != nil {
		return fmt.Errorf("cannot check if Wireguard interface exists: %w", err)
	}
	if exists {
		klog.Infof("Wireguard interface %q already exists", tunnel.TunnelInterfaceName)
		return nil
	}

	if err := createLink(ctx, options); err != nil {
		return fmt.Errorf("cannot create Wireguard interface: %w", err)
	}

	link, err := tunnel.GetLink(tunnel.TunnelInterfaceName)
	if err != nil {
		return fmt.Errorf("cannot get Wireguard interface: %w", err)
	}

	klog.Infof("Setting up Wireguard interface %q with IP %q", tunnel.TunnelInterfaceName, tunnel.GetInterfaceIP(options.GwOptions.Mode))
	if err := tunnel.AddAddress(link, tunnel.GetInterfaceIP(options.GwOptions.Mode)); err != nil {
		return err
	}

	return netlink.LinkSetUp(link)
}

// CreateLink creates a new Wireguard interface.
func createLink(ctx context.Context, options *Options) error {
	var err error
	klog.Infof("Selected wireguard %s implementation", options.Implementation)

	switch options.Implementation {
	case WgImplementationKernel:
		err = createLinkKernel(options)
	case WgImplementationUserspace:
		err = createLinkUserspace(ctx, options)
	default:
		err = fmt.Errorf("invalid wireguard implementation: %s", options.Implementation)
	}

	if err != nil {
		return fmt.Errorf("cannot create Wireguard interface: %w", err)
	}

	if options.GwOptions.Mode == gateway.ModeServer {
		wgcl, err := wgctrl.New()
		if err != nil {
			return fmt.Errorf("cannot create Wireguard client: %w", err)
		}
		defer wgcl.Close()

		if err := wgcl.ConfigureDevice(tunnel.TunnelInterfaceName, wgtypes.Config{
			ListenPort: &options.ListenPort,
		}); err != nil {
			return fmt.Errorf("cannot configure Wireguard interface: %w", err)
		}
	}

	return nil
}

// createLinkKernel creates a new Wireguard interface using the kernel module.
func createLinkKernel(options *Options) error {
	link := netlink.Wireguard{
		LinkAttrs: netlink.LinkAttrs{
			MTU:  options.MTU,
			Name: tunnel.TunnelInterfaceName,
		},
	}

	err := netlink.LinkAdd(&link)
	if err != nil {
		return fmt.Errorf("cannot add Wireguard interface: %w", err)
	}
	return nil
}

// createLinkUserspace creates a new Wireguard interface using the userspace implementation (wireguard-go)
// embedded as a library. The device is kept running in-process for the lifetime of the gateway.
func createLinkUserspace(ctx context.Context, options *Options) error {
	mtu := options.MTU
	if mtu <= 0 {
		mtu = device.DefaultMTU
	}

	tunDev, err := tun.CreateTUN(tunnel.TunnelInterfaceName, mtu)
	if err != nil {
		return fmt.Errorf("failed to create wireguard TUN device %q: %w", tunnel.TunnelInterfaceName, err)
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

func existsLink() (bool, error) {
	_, err := tunnel.GetLink(tunnel.TunnelInterfaceName)
	if err != nil {
		if errors.As(err, &netlink.LinkNotFoundError{}) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
