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

package utils

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

// GetClusterIdentityWithNativeClient returns cluster identity using a kubernetes.Interface client.
func GetClusterIdentityWithNativeClient(ctx context.Context,
	nativeClient kubernetes.Interface, namespace string) (discoveryv1alpha1.ClusterIdentity, error) {
	cmClient := nativeClient.CoreV1().ConfigMaps(namespace)
	configMapList, err := cmClient.List(ctx, metav1.ListOptions{
		LabelSelector: consts.ClusterIDConfigMapSelector().String(),
	})
	if err != nil {
		return discoveryv1alpha1.ClusterIdentity{}, err
	}

	return getClusterIdentityFromConfigMapList(configMapList)
}

// GetClusterIdentityWithControllerClient returns cluster identity using a client.Client client.
func GetClusterIdentityWithControllerClient(ctx context.Context,
	controllerClient client.Client, namespace string) (discoveryv1alpha1.ClusterIdentity, error) {
	var configMapList corev1.ConfigMapList
	if err := controllerClient.List(ctx, &configMapList,
		client.MatchingLabelsSelector{Selector: consts.ClusterIDConfigMapSelector()},
		client.InNamespace(namespace)); err != nil {
		return discoveryv1alpha1.ClusterIdentity{}, fmt.Errorf("%w, unable to get the ClusterID ConfigMap in namespace '%s'", err, namespace)
	}

	return getClusterIdentityFromConfigMapList(&configMapList)
}

// GetClusterName returns the local cluster name.
func GetClusterName(ctx context.Context, k8sClient kubernetes.Interface, namespace string) (string, error) {
	clusterIdentity, err := GetClusterIdentityWithNativeClient(ctx, k8sClient, namespace)
	if err != nil {
		return "", err
	}
	return clusterIdentity.ClusterName, nil
}

// GetClusterID returns the local clusterID.
func GetClusterID(ctx context.Context, cl kubernetes.Interface, namespace string) (string, error) {
	clusterIdentity, err := GetClusterIdentityWithNativeClient(ctx, cl, namespace)
	if err != nil {
		return "", err
	}
	return clusterIdentity.ClusterID, nil
}

func getClusterIdentityFromConfigMapList(configMapList *corev1.ConfigMapList) (discoveryv1alpha1.ClusterIdentity, error) {
	switch len(configMapList.Items) {
	case 0:
		return discoveryv1alpha1.ClusterIdentity{}, apierrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "configmaps",
		}, "clusterid-configmap")
	case 1:
		cm := &configMapList.Items[0]
		clusterID := cm.Data[consts.ClusterIDConfigMapKey]
		klog.V(4).Infof("retrieved ClusterID '%s' from the ConfigMap %q", clusterID, klog.KObj(cm))
		clusterName := cm.Data[consts.ClusterNameConfigMapKey]
		klog.V(4).Infof("retrieved ClusterName '%s' from the ConfigMap %q", clusterName, klog.KObj(cm))
		return discoveryv1alpha1.ClusterIdentity{
			ClusterID:   clusterID,
			ClusterName: clusterName,
		}, nil
	default:
		return discoveryv1alpha1.ClusterIdentity{}, fmt.Errorf("multiple clusterID configmaps found")
	}
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
