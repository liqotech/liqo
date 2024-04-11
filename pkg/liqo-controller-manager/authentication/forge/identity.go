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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

// ControlPlaneIdentityName forges the name of a ControlPlane Identity resource given the remote cluster name.
func ControlPlaneIdentityName(remoteClusterName string) string {
	return "controlplane-" + remoteClusterName
}

// ResourceSliceIdentityName forges the name of a ResourceSlice Identity.
func ResourceSliceIdentityName(resourceSlice *authv1alpha1.ResourceSlice) string {
	return "resourceslice-" + resourceSlice.Name
}

// IdentityForRemoteCluster forges a Identity resource to be applied on a remote cluster.
func IdentityForRemoteCluster(name, namespace string, localClusterIdentity discoveryv1alpha1.ClusterIdentity,
	identityType authv1alpha1.IdentityType, authParams *authv1alpha1.AuthParams, defaultKubeConfigNs *string) *authv1alpha1.Identity {
	identity := Identity(name, namespace)
	MutateIdentity(identity, localClusterIdentity, identityType, authParams, defaultKubeConfigNs)

	return identity
}

// Identity forges a Identity resource.
func Identity(name, namespace string) *authv1alpha1.Identity {
	return &authv1alpha1.Identity{
		TypeMeta: metav1.TypeMeta{
			APIVersion: authv1alpha1.GroupVersion.String(),
			Kind:       authv1alpha1.IdentityKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// MutateIdentity mutates a Identity resource.
func MutateIdentity(identity *authv1alpha1.Identity, remoteClusterIdentity discoveryv1alpha1.ClusterIdentity,
	identityType authv1alpha1.IdentityType, authParams *authv1alpha1.AuthParams, defaultKubeConfigNs *string) {
	if identity.Labels == nil {
		identity.Labels = map[string]string{}
	}
	identity.Labels[consts.RemoteClusterID] = remoteClusterIdentity.ClusterID

	identity.Spec = authv1alpha1.IdentitySpec{
		ClusterIdentity: remoteClusterIdentity,
		Type:            identityType,
		AuthParams:      *authParams,
		Namespace:       defaultKubeConfigNs,
	}
}
