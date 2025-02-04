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

package utils

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/getters"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

// GetVirtualKubeletDeployment returns the VirtualKubelet Deployment of a VirtualNode.
func GetVirtualKubeletDeployment(ctx context.Context, cl client.Client,
	virtualNode *offloadingv1beta1.VirtualNode) (*appsv1.Deployment, error) {
	var deployList appsv1.DeploymentList
	labels := vkforge.VirtualKubeletLabels(virtualNode)
	if err := cl.List(ctx, &deployList, client.MatchingLabels(labels)); err != nil {
		klog.Error(err)
		return nil, err
	}

	if len(deployList.Items) == 0 {
		klog.V(4).Infof("[%v] no VirtualKubelet deployment found", virtualNode.Spec.ClusterID)
		return nil, nil
	} else if len(deployList.Items) > 1 {
		err := fmt.Errorf("[%v] more than one VirtualKubelet deployment found", virtualNode.Spec.ClusterID)
		klog.Error(err)
		return nil, err
	}

	return &deployList.Items[0], nil
}

// CheckVirtualKubeletPodAbsence checks if a VirtualNode's VirtualKubelet pods are absent.
func CheckVirtualKubeletPodAbsence(ctx context.Context, cl client.Client, vn *offloadingv1beta1.VirtualNode) error {
	klog.Infof("[%v] checking virtual-kubelet pod absence", vn.Spec.ClusterID)
	list, err := getters.ListVirtualKubeletPodsFromVirtualNode(ctx, cl, vn)
	if err != nil {
		return err
	}
	klog.Infof("[%v] found %v virtual-kubelet pods", vn.Spec.ClusterID, len(list.Items))
	if len(list.Items) != 0 {
		return fmt.Errorf("[%v] found %v virtual-kubelet pods", vn.Spec.ClusterID, len(list.Items))
	}
	return nil
}

// Flag is a VirtualKubelet flag.
// Name must contain the "--" prefix.
type Flag struct {
	Name  string
	Value string
}

// String returns the flag as a string.
func (f Flag) String() string {
	return fmt.Sprintf("%s=%s", f.Name, f.Value)
}
