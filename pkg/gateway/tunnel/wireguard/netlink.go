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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
)

// InitWireguardLink inits the Wireguard interface.
func InitWireguardLink(ctx context.Context, options *Options, idx int, ports []int) error {
	name := tunnel.GetTunnelName(idx)
	exists, err := existsLink(idx)
	if err != nil {
		return fmt.Errorf("cannot check if Wireguard interface %q exists: %w", name, err)
	}
	if exists {
		klog.Infof("Wireguard interface %q already exists", name)
		return nil
	}

	if err := createLink(ctx, options, idx, ports); err != nil {
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

// CreateLink creates a new Wireguard interface.
func createLink(ctx context.Context, options *Options, idx int, ports []int) error {
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
			ListenPort: &ports[idx],
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

// runWgUserCmd runs the wg command with the given arguments.
func runWgUserCmd(cmd *exec.Cmd) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		outStr, errStr := stdout.String(), stderr.String()
		fmt.Printf("out:\n%s\nerr:\n%s\n", outStr, errStr)
		klog.Fatalf("failed to run '%s': %v", cmd.String(), err)
	}
}

// createLinkUserspace creates a new Wireguard interface using the userspace implementation (wireguard-go).
// TODO: at the moment is not possible to override the settings of the wireguard-go implementation.
// We are planning a PR to add a flag for the MTU.
func createLinkUserspace(ctx context.Context, _ *Options, idx int) error {
	name := tunnel.GetTunnelName(idx)
	cmd := exec.Command("/usr/bin/wireguard-go", "-f", name) //nolint:gosec //we leave it as it is
	go runWgUserCmd(cmd)

	if err := wait.PollUntilContextTimeout(ctx, time.Second, 10*time.Second, true, func(context.Context) (done bool, err error) {
		klog.Infof("Waiting for wireguard device %q to be created", name)
		if _, err = netlink.LinkByName(name); err != nil {
			klog.Errorf("failed to get wireguard device '%s': %s", name, err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("failed to create wireguard device %q: %w", name, err)
	}

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

	return ports
}
