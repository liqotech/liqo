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

package configurationcontroller

import (
	"fmt"

	"k8s.io/apimachinery/pkg/labels"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// LabelCIDRType is the label used to target a ipamv1alpha1.Network resource that manages a PodCIDR or an ExternalCIDR.
const LabelCIDRType = "configuration.liqo.io/cidr-type"

// LabelCIDRTypeValue is the value of the LabelCIDRType label.
type LabelCIDRTypeValue string

const (
	// LabelCIDRTypePod is used to target a ipamv1alpha1.Network resource that manages a PodCIDR.
	LabelCIDRTypePod LabelCIDRTypeValue = "pod"
	// LabelCIDRTypeExternal is used to target a ipamv1alpha1.Network resource that manages an ExternalCIDR.
	LabelCIDRTypeExternal LabelCIDRTypeValue = "external"
)

// LabelCIDRTypeValues is the list of all the possible values of the LabelCIDRType label.
var LabelCIDRTypeValues = []LabelCIDRTypeValue{LabelCIDRTypePod, LabelCIDRTypeExternal}

// ForgeNetworkLabel creates a label to target a ipamv1alpha1.Network resource.
// The label is composed by the remote cluster ID and the CIDR type.
func ForgeNetworkLabel(cfg *networkingv1beta1.Configuration, cidrType LabelCIDRTypeValue) (netLabels map[string]string, err error) {
	remoteClusterID, ok := cfg.Labels[consts.RemoteClusterID]
	if !ok {
		return nil, fmt.Errorf("missing label %s", consts.RemoteClusterID)
	}
	return map[string]string{
		consts.RemoteClusterID: remoteClusterID,
		LabelCIDRType:          string(cidrType),
	}, nil
}

// ForgeNetworkLabelSelector creates a labels.Selector to target a ipamv1alpha1.Network resource.
// The label is composed by the remote cluster ID and the CIDR type.
func ForgeNetworkLabelSelector(cfg *networkingv1beta1.Configuration,
	cidrType LabelCIDRTypeValue) (labelsSelector labels.Selector, err error) {
	result, err := ForgeNetworkLabel(cfg, cidrType)
	if err != nil {
		return nil, err
	}
	return labels.SelectorFromSet(result), nil
}
