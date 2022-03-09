// Copyright 2019-2022 The Liqo Authors
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

// Package labels label selectors used throughout the liqo code in order to get
// k8s resources.
package labels

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

var (
	// IPAMStorageLabelSelector selector used to get the ipam storage instance.
	IPAMStorageLabelSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      liqoconst.IpamStorageResourceLabelKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{liqoconst.IpamStorageResourceLabelValue},
			},
		},
	}

	// GatewayServiceLabelSelector selector used to get the gateway service.
	GatewayServiceLabelSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      liqoconst.GatewayServiceLabelKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{liqoconst.GatewayServiceLabelValue},
			},
		},
	}

	// WireGuardSecretLabelSelector selector used to get the WireGuard secret.
	WireGuardSecretLabelSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      liqoconst.KeysLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{liqoconst.DriverName},
			},
		},
	}

	// ClusterIDConfigMapLabelSelector selector used to get the cluster id configmap.
	ClusterIDConfigMapLabelSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      liqoconst.K8sAppNameKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{liqoconst.ClusterIDConfigMapNameLabelValue},
			},
		},
	}

	// NetworkManagerPodLabelSelector selector used to get the Network Manager Pod.
	NetworkManagerPodLabelSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      liqoconst.K8sAppNameKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{liqoconst.NetworkManagerAppName},
			},
		},
	}

	// AuthServiceLabelSelector selector used to get the auth service.
	AuthServiceLabelSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      liqoconst.K8sAppNameKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{liqoconst.AuthAppName},
			},
		},
	}

	// ProxyServiceLabelSelector selector used to get the gateway service.
	ProxyServiceLabelSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      liqoconst.K8sAppNameKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{liqoconst.APIServerProxyAppName},
			},
		},
	}
)
