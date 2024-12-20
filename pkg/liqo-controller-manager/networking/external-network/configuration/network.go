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

package configurationcontroller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
	"github.com/liqotech/liqo/pkg/utils/events"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// ForgeNetworkMetadata creates the metadata of a ipamv1alpha1.Network resource.
func ForgeNetworkMetadata(net *ipamv1alpha1.Network, cfg *networkingv1beta1.Configuration, cidrType LabelCIDRTypeValue) error {
	labels, err := ForgeNetworkLabel(cfg, cidrType)
	if err != nil {
		return err
	}
	net.Name = fmt.Sprintf("%s-%s", cfg.Name, cidrType)
	net.Namespace = cfg.Namespace
	net.Labels = labels
	return nil
}

// ForgeNetwork creates a ipamv1alpha1.Network resource.
func ForgeNetwork(net *ipamv1alpha1.Network, cfg *networkingv1beta1.Configuration, cidrType LabelCIDRTypeValue,
	scheme *runtime.Scheme) (err error) {
	if err := ForgeNetworkMetadata(net, cfg, cidrType); err != nil {
		return err
	}
	var cidr networkingv1beta1.CIDR
	switch cidrType {
	case LabelCIDRTypePod:
		cidr = *cidrutils.GetPrimary(cfg.Spec.Remote.CIDR.Pod)
	case LabelCIDRTypeExternal:
		cidr = *cidrutils.GetPrimary(cfg.Spec.Remote.CIDR.External)
	}
	net.Spec = ipamv1alpha1.NetworkSpec{
		CIDR: cidr,
	}
	err = ctrlutil.SetControllerReference(cfg, net, scheme)
	if err != nil {
		return err
	}
	return nil
}

// CreateOrGetNetwork creates or gets a ipamv1alpha1.Network resource.
func CreateOrGetNetwork(ctx context.Context, cl client.Client, scheme *runtime.Scheme, er record.EventRecorder,
	cfg *networkingv1beta1.Configuration, cidrType LabelCIDRTypeValue) (*ipamv1alpha1.Network, error) {
	ls, err := ForgeNetworkLabelSelector(cfg, cidrType)
	if err != nil {
		return nil, err
	}
	ns := cfg.Namespace
	list, err := getters.ListNetworksByLabel(ctx, cl, ns, ls)
	if err != nil {
		return nil, err
	}
	if len(list.Items) == 1 {
		if len(list.Items[0].OwnerReferences) == 1 && string(list.Items[0].OwnerReferences[0].UID) == string(cfg.UID) {
			return &list.Items[0], nil
		}
	}
	if len(list.Items) > 1 {
		return nil, fmt.Errorf("multiple networks found with label selector '%s'", ls)
	}

	events.Event(er, cfg, fmt.Sprintf("Creating network %s/%s", cfg.Name, cfg.Namespace))

	network := &ipamv1alpha1.Network{}
	if err = ForgeNetworkMetadata(network, cfg, cidrType); err != nil {
		return nil, err
	}

	if _, err := resource.CreateOrUpdate(ctx, cl, network, func() error {
		return ForgeNetwork(network, cfg, cidrType, scheme)
	}); err != nil {
		return nil, err
	}

	events.Event(er, cfg, fmt.Sprintf("Network %s/%s created", cfg.Name, cfg.Namespace))
	return network, nil
}
