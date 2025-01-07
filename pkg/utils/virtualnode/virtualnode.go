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

package virtualnode

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// ForgeFakeNodeFromVirtualNode creates a fake node from a virtual node.
func ForgeFakeNodeFromVirtualNode(ctx context.Context, cl client.Client, vn *offloadingv1beta1.VirtualNode) (*corev1.Node, error) {
	l, err := GetLabelSelectors(ctx, cl, vn)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve label selectors: %w", err)
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   vn.Name,
			Labels: l,
		},
	}, nil
}

// GetLabelSelectors returns the labels that can be used to target a remote cluster.
// If virtualnode spec.CreateNode is true, the labels are taken from the created node.
// If virtualnode spec.CreateNode is false, the labels are taken from the virtual node spec.Labels and spec.Template .
func GetLabelSelectors(ctx context.Context, cl client.Client, vn *offloadingv1beta1.VirtualNode) (labels.Set, error) {
	n, err := getters.GetNodeFromVirtualNode(ctx, cl, vn)
	switch {
	case errors.IsNotFound(err):
		return labels.Merge(vn.Spec.Labels, labels.Set{
			liqoconsts.RemoteClusterID:       string(vn.Spec.ClusterID),
			liqoconsts.StorageAvailableLabel: strconv.FormatBool(len(vn.Spec.StorageClasses) == 0),
		}), nil
	case err != nil:
		return nil, fmt.Errorf("failed to retrieve node %s from VirtualNode %s: %w", n.Name, vn.Name, err)
	default:
		return n.Labels, nil
	}
}

// GetVirtualNodeClusterID returns the clusterID given a virtual node.
func GetVirtualNodeClusterID(vn *offloadingv1beta1.VirtualNode) (liqov1beta1.ClusterID, bool) {
	remoteClusterID := vn.Spec.ClusterID
	if remoteClusterID == "" {
		return "", false
	}

	return remoteClusterID, true
}
