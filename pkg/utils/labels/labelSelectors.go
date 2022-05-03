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
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

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

// LocalLabelSelector returns a label selector to match local resources.
func LocalLabelSelector() labels.Selector {
	req, err := labels.NewRequirement(liqoconst.ReplicationRequestedLabel, selection.Equals, []string{strconv.FormatBool(true)})
	utilruntime.Must(err)

	return labels.NewSelector().Add(*req)
}

// RemoteLabelSelector returns a label selector to match local resources.
func RemoteLabelSelector() labels.Selector {
	req, err := labels.NewRequirement(liqoconst.ReplicationStatusLabel, selection.Equals, []string{strconv.FormatBool(true)})
	utilruntime.Must(err)

	return labels.NewSelector().Add(*req)
}

// LocalLabelSelectorForCluster returns a label selector to match local resources with a given destination ClusterID.
func LocalLabelSelectorForCluster(destinationClusterID string) labels.Selector {
	req, err := labels.NewRequirement(liqoconst.ReplicationDestinationLabel, selection.Equals, []string{destinationClusterID})
	utilruntime.Must(err)

	return LocalLabelSelector().Add(*req)
}

// RemoteLabelSelectorForCluster returns a label selector to match remote resources with a given origin ClusterID.
func RemoteLabelSelectorForCluster(originClusterID string) labels.Selector {
	req, err := labels.NewRequirement(liqoconst.ReplicationOriginLabel, selection.Equals, []string{originClusterID})
	utilruntime.Must(err)

	return RemoteLabelSelector().Add(*req)
}
