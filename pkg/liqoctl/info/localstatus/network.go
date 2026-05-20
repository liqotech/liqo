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
//

package localstatus

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/ipam"
)

// Network represents the status of the network of the local Liqo installation.
type Network struct {
	PodCIDRs      []string `json:"podCIDRs"`
	ServiceCIDR   string   `json:"serviceCIDR"`
	ExternalCIDRs []string `json:"externalCIDRs"`
	InternalCIDR  string   `json:"internalCIDR"`
}

// NetworkChecker collects info about the local installation of Liqo.
type NetworkChecker struct {
	info.CheckerCommon
	data Network
}

// Collect data about the network of the local installation of Liqo.
func (l *NetworkChecker) Collect(ctx context.Context, options info.Options) {
	listFields := map[string]func(ctx context.Context, cl client.Client, namespace string) ([]string, error){
		"PodCIDRs":      ipam.GetPodCIDRs,
		"ExternalCIDRs": ipam.GetExternalCIDRs,
	}
	scalarFields := map[string]func(ctx context.Context, cl client.Client, namespace string) (string, error){
		"ServiceCIDR":  ipam.GetServiceCIDR,
		"InternalCIDR": ipam.GetInternalCIDR,
	}

	for key, fn := range listFields {
		val, err := fn(ctx, options.CRClient, corev1.NamespaceAll)
		if err != nil {
			l.AddCollectionError(fmt.Errorf("unable to get %s: %w", key, err))
			continue
		}
		switch key {
		case "PodCIDRs":
			l.data.PodCIDRs = val
		case "ExternalCIDRs":
			l.data.ExternalCIDRs = val
		}
	}

	for key, fn := range scalarFields {
		val, err := fn(ctx, options.CRClient, corev1.NamespaceAll)
		if err != nil {
			l.AddCollectionError(fmt.Errorf("unable to get %s: %w", key, err))
			continue
		}
		switch key {
		case "ServiceCIDR":
			l.data.ServiceCIDR = val
		case "InternalCIDR":
			l.data.InternalCIDR = val
		}
	}
}

// Format returns the collected data using a user friendly output.
func (l *NetworkChecker) Format(options info.Options) string {
	main := output.NewRootSection()
	main.AddEntry("Pod CIDRs", l.data.PodCIDRs...)
	main.AddEntry("Service CIDR", l.data.ServiceCIDR)
	main.AddEntry("External CIDRs", l.data.ExternalCIDRs...)
	main.AddEntry("Internal CIDR", l.data.InternalCIDR)

	return main.SprintForBox(options.Printer)
}

// GetData returns the data collected by the checker.
func (l *NetworkChecker) GetData() interface{} {
	return l.data
}

// GetID returns the id of the section collected by the checker.
func (l *NetworkChecker) GetID() string {
	return "network"
}

// GetTitle returns the title of the section collected by the checker.
func (l *NetworkChecker) GetTitle() string {
	return "Network"
}
