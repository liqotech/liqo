// Copyright 2019-2023 The Liqo Authors
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

package configuration

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
)

// ForgeConfigurationForRemoteCluster forges a configuration of the local cluster to be applied to a remote cluster.
// It retrieves the local configuration settings starting from the cluster identity and the IPAM storage.
func ForgeConfigurationForRemoteCluster(ctx context.Context, cl client.Client,
	namespace, liqoNamespace string) (*networkingv1alpha1.Configuration, error) {
	clusterIdentity, err := liqoutils.GetClusterIdentityWithControllerClient(ctx, cl, liqoNamespace)
	if err != nil {
		return nil, fmt.Errorf("unable to get cluster identity: %w", err)
	}

	ipamStorage, err := liqogetters.GetIPAMStorageByLabel(ctx, cl, labels.NewSelector())
	if err != nil {
		return nil, fmt.Errorf("unable to get IPAM storage: %w", err)
	}

	cnf := &networkingv1alpha1.Configuration{
		TypeMeta: metav1.TypeMeta{
			Kind:       networkingv1alpha1.ConfigurationKind,
			APIVersion: networkingv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultConfigurationName(&clusterIdentity),
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: clusterIdentity.ClusterID,
			},
		},
		Spec: networkingv1alpha1.ConfigurationSpec{
			Remote: networkingv1alpha1.ClusterConfig{
				CIDR: networkingv1alpha1.ClusterConfigCIDR{
					Pod:      networkingv1alpha1.CIDR(ipamStorage.Spec.PodCIDR),
					External: networkingv1alpha1.CIDR(ipamStorage.Spec.ExternalCIDR),
				},
			},
		},
	}

	if namespace != "" && namespace != corev1.NamespaceDefault {
		cnf.Namespace = namespace
	}
	return cnf, nil
}

// DefaultConfigurationName returns the default name for a Configuration.
func DefaultConfigurationName(remoteClusterIdentity *discoveryv1alpha1.ClusterIdentity) string {
	return remoteClusterIdentity.ClusterName
}

// IsConfigurationStatusSet check if a Configuration is ready by checking if its status is correctly set.
func IsConfigurationStatusSet(ctx context.Context, cl client.Client, name, namespace string) (bool, error) {
	conf := &networkingv1alpha1.Configuration{}
	if err := cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, conf); err != nil {
		return false, err
	}

	return conf.Status.Remote != nil &&
			conf.Status.Remote.CIDR.Pod.String() != "" &&
			conf.Status.Remote.CIDR.External.String() != "",
		nil
}
