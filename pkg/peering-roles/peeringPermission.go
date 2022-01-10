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

package peeringroles

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// These labels are assigned to the ClusterRoles through the Helm chart.
// In case a change is performed here, the modification must be propagated to the template definition.
const (
	// remotePermissionsLabelKey -> the label key used to identify the cluster roles associated with peering permissions.
	remotePermissionsLabelKey = "auth.liqo.io/remote-peering-permissions"
	// remotePermissionsLabelBasic -> the label value identifying basic peering permissions.
	remotePermissionsLabelBasic = "basic"
	// remotePermissionsLabelIncoming -> the label value identifying incoming peering permissions.
	remotePermissionsLabelIncoming = "incoming"
	// remotePermissionsLabelOutgoing -> the label value identifying outgoing peering permissions.
	remotePermissionsLabelOutgoing = "outgoing"
)

// PeeringPermission contains the reference to the ClusterRoles
// to bind in the different peering phases.
type PeeringPermission struct {
	// to be enabled with the creation of the Tenant Namespace,
	// these ClusterRoles have the basic permissions to give to a remote cluster
	Basic []*rbacv1.ClusterRole
	// to be enabled when a ResourceRequest has been accepted,
	// these ClusterRoles have the permissions required by a remote cluster
	// to manage an outgoing peering (incoming for the local cluster),
	// when the Pods will be offloaded to the local cluster
	Incoming []*rbacv1.ClusterRole
	// to be enabled when we send a ResourceRequest,
	// these ClusterRoles have the permissions required by a remote cluster
	// to manage an incoming peering (outgoing for the local cluster),
	// when the Pods will be offloaded from the local cluster
	Outgoing []*rbacv1.ClusterRole
}

// GetPeeringPermission populates a PeeringPermission with the ClusterRole names provided by the configuration.
func GetPeeringPermission(ctx context.Context, client kubernetes.Interface) (*PeeringPermission, error) {
	basic, err := getClusterRoles(ctx, client, remotePermissionsLabelSelector(remotePermissionsLabelBasic))
	if err != nil {
		return nil, err
	}

	incoming, err := getClusterRoles(ctx, client, remotePermissionsLabelSelector(remotePermissionsLabelIncoming))
	if err != nil {
		return nil, err
	}

	outgoing, err := getClusterRoles(ctx, client, remotePermissionsLabelSelector(remotePermissionsLabelOutgoing))
	if err != nil {
		return nil, err
	}

	return &PeeringPermission{
		Basic:    basic,
		Incoming: incoming,
		Outgoing: outgoing,
	}, nil
}

// getClusterRoles gets a set of ClusterRoles given a label selector.
func getClusterRoles(ctx context.Context, client kubernetes.Interface, selector labels.Selector) ([]*rbacv1.ClusterRole, error) {
	clusterroleslist, err := client.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		klog.Error("Failed to retrieve ClusterRoles: %w", err)
		return nil, err
	}

	output := make([]*rbacv1.ClusterRole, len(clusterroleslist.Items))
	for i := range clusterroleslist.Items {
		output[i] = &clusterroleslist.Items[i]
	}
	return output, nil
}

// remotePermissionsLabelSelector returns a label selector matching the custer roles including the permissions for the given level.
func remotePermissionsLabelSelector(level string) labels.Selector {
	req, err := labels.NewRequirement(remotePermissionsLabelKey, selection.Equals, []string{level})
	utilruntime.Must(err)
	return labels.NewSelector().Add(*req)
}
