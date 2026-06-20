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
	"time"

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
	"github.com/liqotech/liqo/pkg/utils/kernel"
)

// InitWireguardLink inits the Wireguard interface.
func InitWireguardLink(ctx context.Context, options *Options, idx int) error {
	name := tunnel.GetTunnelName(idx)
	exists, err := existsLink(name)
	if err != nil {
		return fmt.Errorf("checking if Wireguard interface %q exists: %w", name, err)
	}
	if exists {
		klog.Infof("Wireguard interface %q already exists", name)
		return nil
	}

	if err := createLink(ctx, options, idx); err != nil {
		return fmt.Errorf("creating Wireguard interface %q: %w", name, err)
	}

	link, err := tunnel.GetLink(name)
	if err != nil {
		return fmt.Errorf("getting Wireguard interface %q: %w", name, err)
	}

	klog.Infof("Setting up Wireguard interface %q with IP %q", name, tunnel.GetInterfaceIP(options.GwOptions.Mode, idx))
	if err := tunnel.AddAddress(link, tunnel.GetInterfaceIP(options.GwOptions.Mode, idx)); err != nil {
		return err
	}

	return netlink.LinkSetUp(link)
}

// createLink creates a new Wireguard interface.
func createLink(ctx context.Context, options *Options, idx int) error {
	var err error
	name := tunnel.GetTunnelName(idx)
	klog.Infof("Selected wireguard %s implementation", options.Implementation)

	switch options.Implementation {
	case WgImplementationKernel:
		err = createLinkKernel(options, name)
	case WgImplementationUserspace:
		err = createLinkUserspace(ctx, options, name)
	default:
		err = fmt.Errorf("invalid wireguard implementation: %s", options.Implementation)
	}

	if err != nil {
		return fmt.Errorf("cannot create Wireguard interface %q: %w", name, err)
	}

	if options.GwOptions.Mode == gateway.ModeServer {
		wgcl, err := wgctrl.New()
		if err != nil {
			return fmt.Errorf("cannot create Wireguard client (interface %q): %w", name, err)
		}
		defer wgcl.Close()

		if err := wgcl.ConfigureDevice(name, wgtypes.Config{
			ListenPort: &options.ListenPorts[idx],
		}); err != nil {
			return fmt.Errorf("cannot configure Wireguard interface %q: %w", name, err)
		}
	}

	return nil
}

// createLinkKernel creates a new Wireguard interface using the kernel module.
func createLinkKernel(options *Options, name string) error {
	link := netlink.Wireguard{
		LinkAttrs: netlink.LinkAttrs{
			MTU:  options.MTU,
			Name: name,
		},
	}

	err := netlink.LinkAdd(&link)
	if err != nil {
		return fmt.Errorf("getting Wireguard interface %q: %w", name, err)
	}
	return nil
}

// createLinkUserspace creates a new Wireguard interface using the userspace implementation (wireguard-go)
// embedded as a library. The device is kept running in-process for the lifetime of the gateway.
func createLinkUserspace(ctx context.Context, options *Options, intName string) error {
	mtu := options.MTU
	if mtu <= 0 {
		mtu = device.DefaultMTU
	}

	tunDev, err := tun.CreateTUN(intName, mtu)
	if err != nil {
		return fmt.Errorf("creating wireguard TUN device %q: %w", intName, err)
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

func existsLink(name string) (bool, error) {
	_, err := tunnel.GetLink(name)
	if err != nil {
		if errors.As(err, &netlink.LinkNotFoundError{}) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetWireguardPorts returns the list of ports to be used for WireGuard interfaces.
// The number of ports must not exceed tunnel.MaxWireguardInterfaces, otherwise an error is returned.
func GetWireguardPorts(opts *Options) ([]int, error) {
	var ports []int

	switch opts.GwOptions.Mode {
	case gateway.ModeClient:
		ports = opts.EndpointPorts
	case gateway.ModeServer:
		ports = opts.ListenPorts
	default:
		return nil, fmt.Errorf("invalid mode %v", opts.GwOptions.Mode)
	}
	if len(ports) > tunnel.MaxWireguardInterfaces {
		return nil, fmt.Errorf("requested %d WireGuard interfaces, maximum allowed is %d", len(ports), tunnel.MaxWireguardInterfaces)
	}

	return ports, nil
}

// EnsureThreadedNAPI enables threaded NAPI for all WireGuard interfaces.
// Retry up to maxNAPIAttempts times per interface to handle transient failures.
func EnsureThreadedNAPI(interfaces int) error {
	if err := kernel.IsThreadedNAPISupported(tunnel.GetTunnelName(0)); err != nil {
		return fmt.Errorf("checking kernel support: %w", err)
	}

	if err := kernel.RemountSysfsRW(); err != nil {
		return fmt.Errorf("remounting sysfs as R/W: %w", err)
	}
	defer kernel.RemountSysfsRO()

	for i := range interfaces {
		name := tunnel.GetTunnelName(i)

		var err error
		for attempt := range maxNAPIAttempts {
			if attempt > 0 {
				time.Sleep(time.Duration(attempt) * napiBackoffBase)
			}
			var changed bool
			changed, err = kernel.EnableWireguardThreadedMode(name)
			if err == nil {
				if changed {
					klog.Infof("threaded NAPI enabled for interface %s (attempt %d/%d)", name, attempt+1, maxNAPIAttempts)
				} else {
					klog.Infof("threaded NAPI already enabled for interface %s", name)
				}
				break
			}
			klog.Infof("unable to enable threaded NAPI for interface %s (attempt %d/%d): %v", name, attempt+1, maxNAPIAttempts, err)
		}

		if err != nil {
			return fmt.Errorf("configuring interface %s failed after %d attempts: %w", name, maxNAPIAttempts, err)
		}
	}

	return nil
}
