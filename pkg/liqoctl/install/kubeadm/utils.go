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

package kubeadm

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqoctl/common"
)

func retrieveClusterParameters(ctx context.Context, client kubernetes.Interface) (podCIDR, serviceCIDR string, err error) {
	kubeControllerSpec, err := client.CoreV1().Pods(kubeSystemNamespaceName).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(kubeControllerManagerLabels).AsSelector().String(),
	})
	if err != nil {
		return "", "", err
	}
	if len(kubeControllerSpec.Items) < 1 {
		return "", "", fmt.Errorf("kube-controller-manager not found")
	}
	if len(kubeControllerSpec.Items[0].Spec.Containers) != 1 {
		return "", "", fmt.Errorf("unexpected amount of containers in kube-controller-manager")
	}
	command := kubeControllerSpec.Items[0].Spec.Containers[0].Command
	podCIDR, err = common.ExtractValueFromArgumentList(podCIDRParameterFilter, command)
	klog.V(4).Infof("Extracted podCIDR: %s\n", podCIDR)
	if err != nil {
		return "", "", err
	}
	serviceCIDR, err = common.ExtractValueFromArgumentList(serviceCIDRParameterFilter, command)
	klog.V(4).Infof("Extracted serviceCIDR: %s\n", serviceCIDR)
	if err != nil {
		return "", "", err
	}
	return podCIDR, serviceCIDR, nil
}
