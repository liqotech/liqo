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

package fabricipam

import (
	"context"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// Init initializes the IPAM.
// It lists all the internalnode and internalfabric resources and configures the IPAM accordingly.
func Init(ctx context.Context, cl client.Client, ipam *IPAM) error {
	if err := initInternalNodes(ctx, cl, ipam); err != nil {
		return err
	}
	return initInternalFabrics(ctx, cl, ipam)
}

// initInternalNodes initializes the IPAM with the internal nodes.
func initInternalNodes(ctx context.Context, cl client.Client, ipam *IPAM) error {
	list := &networkingv1beta1.InternalNodeList{}
	if err := cl.List(ctx, list); err != nil {
		return err
	}

	for i := range list.Items {
		internalNode := &list.Items[i]
		if ipam.isIPConfigured(internalNode.Spec.Interface.Node.IP.String()) {
			continue
		}
		klog.Infof("Configuring fabric IPAM for internal node %s: %s", internalNode.Name, internalNode.Spec.Interface.Node.IP.String())
		if err := ipam.configure(internalNode.Name, internalNode.Spec.Interface.Node.IP.String()); err != nil {
			return err
		}
	}
	return nil
}

// initInternalFabrics initializes the IPAM with the internal fabrics.
func initInternalFabrics(ctx context.Context, cl client.Client, ipam *IPAM) error {
	list := &networkingv1beta1.InternalFabricList{}
	if err := cl.List(ctx, list); err != nil {
		return err
	}
	for i := range list.Items {
		internalFabric := &list.Items[i]
		if ipam.isIPConfigured(internalFabric.Spec.Interface.Gateway.IP.String()) {
			continue
		}
		klog.Infof("Configuring fabric IPAM for internal fabric %s: %s", internalFabric.Name, internalFabric.Spec.Interface.Gateway.IP.String())
		if err := ipam.configure(internalFabric.Name, internalFabric.Spec.Interface.Gateway.IP.String()); err != nil {
			return err
		}
	}
	return nil
}
