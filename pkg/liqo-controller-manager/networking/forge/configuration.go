// Copyright 2019-2024 The Liqo Authors
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

package forge

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1alpha1 "github.com/liqotech/liqo/apis/core/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
)

// DefaultConfigurationName returns the default name for a Configuration.
func DefaultConfigurationName(remoteClusterID liqov1alpha1.ClusterID) string {
	return string(remoteClusterID)
}

// Configuration forges a Configuration resource of a remote cluster.
func Configuration(name, namespace string, remoteClusterID liqov1alpha1.ClusterID,
	podCIDR, externalCIDR string) *networkingv1alpha1.Configuration {
	conf := &networkingv1alpha1.Configuration{
		TypeMeta: metav1.TypeMeta{
			Kind:       networkingv1alpha1.ConfigurationKind,
			APIVersion: networkingv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				consts.RemoteClusterID: string(remoteClusterID),
			},
		},
	}
	MutateConfiguration(conf, remoteClusterID, podCIDR, externalCIDR)
	return conf
}

// MutateConfiguration mutates a Configuration resource of a remote cluster.
func MutateConfiguration(conf *networkingv1alpha1.Configuration, remoteClusterID liqov1alpha1.ClusterID, podCIDR, externalCIDR string) {
	conf.Kind = networkingv1alpha1.ConfigurationKind
	conf.APIVersion = networkingv1alpha1.GroupVersion.String()
	if conf.Labels == nil {
		conf.Labels = make(map[string]string)
	}
	conf.Labels[consts.RemoteClusterID] = string(remoteClusterID)
	conf.Spec.Remote.CIDR.Pod = networkingv1alpha1.CIDR(podCIDR)
	conf.Spec.Remote.CIDR.External = networkingv1alpha1.CIDR(externalCIDR)
}

// ConfigurationForRemoteCluster forges a Configuration of the local cluster to be applied to a remote cluster.
// It retrieves the local configuration settings starting from the cluster identity and the IPAM storage.
func ConfigurationForRemoteCluster(ctx context.Context, cl client.Client,
	namespace, liqoNamespace string) (*networkingv1alpha1.Configuration, error) {
	clusterID, err := liqoutils.GetClusterIDWithControllerClient(ctx, cl, liqoNamespace)
	if err != nil {
		return nil, fmt.Errorf("unable to get cluster identity: %w", err)
	}

	podCIDR, err := ipamutils.GetPodCIDR(ctx, cl)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve pod CIDR: %w", err)
	}

	externalCIDR, err := ipamutils.GetExternalCIDR(ctx, cl)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve external CIDR: %w", err)
	}

	cnf := &networkingv1alpha1.Configuration{
		TypeMeta: metav1.TypeMeta{
			Kind:       networkingv1alpha1.ConfigurationKind,
			APIVersion: networkingv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultConfigurationName(clusterID),
			Labels: map[string]string{
				consts.RemoteClusterID: string(clusterID),
			},
		},
		Spec: networkingv1alpha1.ConfigurationSpec{
			Remote: networkingv1alpha1.ClusterConfig{
				CIDR: networkingv1alpha1.ClusterConfigCIDR{
					Pod:      networkingv1alpha1.CIDR(podCIDR),
					External: networkingv1alpha1.CIDR(externalCIDR),
				},
			},
		},
	}

	if namespace != "" && namespace != corev1.NamespaceDefault {
		cnf.Namespace = namespace
	}
	return cnf, nil
}
