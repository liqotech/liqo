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

	// GatewayResourceLabelSelector selector is used to get resources related to a gateway.
	GatewayResourceLabelSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      liqoconst.GatewayResourceLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{liqoconst.GatewayResourceLabelValue},
			},
		},
	}

	// ResourceForRemoteClusterLabelSelector selector is used to get resources related to a remote cluster.
	ResourceForRemoteClusterLabelSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      liqoconst.RemoteClusterID,
				Operator: metav1.LabelSelectorOpExists,
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

// ComponentLabelSelector returns the label selector associated with the component characterized by the given name and component labels.
func ComponentLabelSelector(name, component string) labels.Selector {
	// These labels are configured through Helm at install time.
	req1, err := labels.NewRequirement(liqoconst.K8sAppNameKey, selection.Equals, []string{name})
	utilruntime.Must(err)
	req2, err := labels.NewRequirement(liqoconst.K8sAppComponentKey, selection.Equals, []string{component})
	utilruntime.Must(err)

	return labels.NewSelector().Add(*req1, *req2)
}

// ControllerManagerLabelSelector returns the label selector associated with the controller-manager components.
func ControllerManagerLabelSelector() labels.Selector {
	return ComponentLabelSelector("controller-manager", "controller-manager")
}
