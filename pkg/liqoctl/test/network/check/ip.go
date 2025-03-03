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

package check

import (
	"context"
	"fmt"
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientctrl "sigs.k8s.io/controller-runtime/pkg/client"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/setup"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
	"github.com/liqotech/liqo/pkg/utils/ipam/mapping"
)

// RunChecksPodToExternalRemappedIP runs all the checks from the pod to the external remapped IP.
func RunChecksPodToExternalRemappedIP(ctx context.Context, cl *client.Client,
	cfg client.Configs, opts *flags.Options) (successCount, errorCount int32, err error) {
	var successCountTot, errorCountTot int32

	targets, err := ForgeIPTargets(ctx, cl)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to forge targets: %w", err)
	}

	successCount, errorCount, err = RunCheckToTargets(ctx, cl.Consumer, cfg[cl.ConsumerName],
		opts, cl.ConsumerName, targets[cl.ConsumerName], false, ExecNetcatTCPConnect)
	successCountTot += successCount
	errorCountTot += errorCount
	if err != nil {
		return successCountTot, errorCountTot, fmt.Errorf("consumer failed to run checks: %w", err)
	}

	for k := range cl.Providers {
		successCount, errorCount, err := RunCheckToTargets(ctx, cl.Providers[k], cfg[k],
			opts, k, targets[k], false, ExecNetcatTCPConnect)
		successCountTot += successCount
		errorCountTot += errorCount
		if err != nil {
			return successCountTot, errorCountTot, fmt.Errorf("provider %q failed to run checks: %w", k, err)
		}
	}

	return successCountTot, errorCountTot, nil
}

// ForgeIPTargets forges the IP targets for the consumer and the providers.
func ForgeIPTargets(ctx context.Context, cl *client.Client) (Targets, error) {
	localIPRemapped, err := GetLocalIPRemapped(ctx, cl)
	if err != nil {
		return nil, fmt.Errorf("failed to get local IP remapped: %w", err)
	}

	targets := Targets{}

	targets[cl.ConsumerName], err = forgeIPTarget(ctx, cl.Consumer, localIPRemapped, cl.ConsumerName)
	if err != nil {
		return nil, fmt.Errorf("failed to forge consumer IP target: %w", err)
	}

	for k := range cl.Providers {
		targets[k], err = forgeIPTarget(ctx, cl.Providers[k], localIPRemapped, k)
		if err != nil {
			return nil, fmt.Errorf("failed to forge provider %q IP target: %w", k, err)
		}
	}

	return targets, nil
}

func forgeIPTarget(ctx context.Context, cl clientctrl.Client, localIPRemapped map[string]string, localName string) ([]string, error) {
	target := []string{
		localIPRemapped[localName],
	}

	cfgs := networkingv1beta1.ConfigurationList{}
	if err := cl.List(ctx, &cfgs); err != nil {
		return nil, fmt.Errorf("failed to list configurations: %w", err)
	}

	for i := range cfgs.Items {
		if cfgs.Items[i].Labels == nil {
			continue
		}

		id, ok := cfgs.Items[i].Labels[consts.RemoteClusterID]
		if !ok {
			continue
		}

		ip, ok := localIPRemapped[id]
		if !ok {
			return nil, fmt.Errorf("failed to get IP target for remote cluster %q", id)
		}

		ipnet := net.ParseIP(ip)
		if ipnet == nil {
			return nil, fmt.Errorf("failed to parse IP: %s", ip)
		}

		_, cidrtarget, err := net.ParseCIDR(cidrutils.GetPrimary(cfgs.Items[i].Status.Remote.CIDR.External).String())
		if err != nil {
			return nil, fmt.Errorf("failed to parse CIDR: %w", err)
		}

		needsRemap := !cidrtarget.Contains(net.ParseIP(ip))
		if !needsRemap {
			target = append(target, ipnet.String())
		} else {
			ipnet = mapping.RemapMask(ipnet, *cidrtarget)
			target = append(target, ipnet.String())
		}
	}
	return target, nil
}

// GetLocalIPRemapped gets the local IP remapped for the consumer and the providers.
func GetLocalIPRemapped(ctx context.Context, cl *client.Client) (map[string]string, error) {
	var localIPRemapped = make(map[string]string)
	ip := ipamv1alpha1.IP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      setup.IPName,
			Namespace: setup.NamespaceName,
		},
	}

	if err := cl.Consumer.Get(ctx, clientctrl.ObjectKeyFromObject(&ip), &ip); err != nil {
		return nil, fmt.Errorf("failed to get consumer IP: %w", err)
	}

	v := ipamutils.GetRemappedIP(&ip)
	localIPRemapped[cl.ConsumerName] = v.String()

	for providerName := range cl.Providers {
		ip := ipamv1alpha1.IP{
			ObjectMeta: metav1.ObjectMeta{
				Name:      setup.IPName,
				Namespace: setup.NamespaceName,
			},
		}

		if err := cl.Providers[providerName].Get(ctx, clientctrl.ObjectKeyFromObject(&ip), &ip); err != nil {
			return nil, fmt.Errorf("failed to get provider %q IP: %w", providerName, err)
		}

		v := ipamutils.GetRemappedIP(&ip)
		localIPRemapped[providerName] = v.String()
	}

	return localIPRemapped, nil
}
