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

package forge

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// ControlPlaneIdentityName forges the name of a ControlPlane Identity resource given the remote cluster name.
func ControlPlaneIdentityName(remoteClusterID liqov1beta1.ClusterID) string {
	return "controlplane-" + string(remoteClusterID)
}

// ResourceSliceIdentityName forges the name of a ResourceSlice Identity.
func ResourceSliceIdentityName(resourceSlice *authv1beta1.ResourceSlice) string {
	return "resourceslice-" + resourceSlice.Name
}

// IdentityForRemoteCluster forges a Identity resource to be applied on a remote cluster.
func IdentityForRemoteCluster(name, namespace string, localClusterID liqov1beta1.ClusterID,
	identityType authv1beta1.IdentityType, authParams *authv1beta1.AuthParams, defaultKubeConfigNs *string) *authv1beta1.Identity {
	identity := Identity(name, namespace)
	MutateIdentity(identity, localClusterID, identityType, authParams, defaultKubeConfigNs)

	return identity
}

// Identity forges a Identity resource.
func Identity(name, namespace string) *authv1beta1.Identity {
	return &authv1beta1.Identity{
		TypeMeta: metav1.TypeMeta{
			APIVersion: authv1beta1.GroupVersion.String(),
			Kind:       authv1beta1.IdentityKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// MutateIdentity mutates a Identity resource.
func MutateIdentity(identity *authv1beta1.Identity, remoteClusterID liqov1beta1.ClusterID,
	identityType authv1beta1.IdentityType, authParams *authv1beta1.AuthParams, defaultKubeConfigNs *string) {
	if identity.Labels == nil {
		identity.Labels = map[string]string{}
	}
	identity.Labels[consts.RemoteClusterID] = string(remoteClusterID)

	identity.Spec = authv1beta1.IdentitySpec{
		ClusterID:  remoteClusterID,
		Type:       identityType,
		AuthParams: *authParams,
		Namespace:  defaultKubeConfigNs,
	}
}
