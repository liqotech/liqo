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
	"os"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

// GetClusterIDWithNativeClient returns cluster identity using a kubernetes.Interface client.
func GetClusterIDWithNativeClient(ctx context.Context,
	nativeClient kubernetes.Interface, namespace string) (liqov1beta1.ClusterID, error) {
	cmClient := nativeClient.CoreV1().ConfigMaps(namespace)
	configMapList, err := cmClient.List(ctx, metav1.ListOptions{
		LabelSelector: consts.ClusterIDConfigMapSelector().String(),
	})
	if err != nil {
		return "", err
	}

	switch len(configMapList.Items) {
	case 0:
		return "", apierrors.NewNotFound(
			corev1.Resource(corev1.ResourceConfigMaps.String()),
			consts.ClusterIDConfigMapNameLabelValue)
	case 1:
		clusterID, err := liqogetters.RetrieveClusterIDFromConfigMap(&configMapList.Items[0])
		return clusterID, err
	default:
		return "", fmt.Errorf("multiple clusterID configmaps found")
	}
}

// GetClusterIDWithControllerClient returns cluster identity using a client.Client client.
func GetClusterIDWithControllerClient(ctx context.Context, cl client.Client, namespace string) (liqov1beta1.ClusterID, error) {
	selector, err := metav1.LabelSelectorAsSelector(&liqolabels.ClusterIDConfigMapLabelSelector)
	if err != nil {
		return "", err
	}
	cm, err := liqogetters.GetConfigMapByLabel(ctx, cl, namespace, selector)
	if err != nil {
		return "", err
	}
	clusterID, err := liqogetters.RetrieveClusterIDFromConfigMap(cm)
	if err != nil {
		return "", err
	}
	return clusterID, nil
}

// GetClusterID returns the local clusterID.
func GetClusterID(ctx context.Context, cl kubernetes.Interface, namespace string) (liqov1beta1.ClusterID, error) {
	clusterID, err := GetClusterIDWithNativeClient(ctx, cl, namespace)
	if err != nil {
		return "", err
	}
	return clusterID, nil
}

// GetRestConfig returns a rest.Config object to initialize a client to the target cluster.
func GetRestConfig(configPath string) (config *rest.Config, err error) {
	if _, err = os.Stat(configPath); err == nil {
		// Get the kubeconfig from the filepath.
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
	} else {
		// Set to in-cluster config.
		config, err = rest.InClusterConfig()
	}
	return config, err
}
