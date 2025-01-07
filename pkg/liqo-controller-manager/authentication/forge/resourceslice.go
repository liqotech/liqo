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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// ResourceSliceOptions contains the options to forge a ResourceSlice resource.
type ResourceSliceOptions struct {
	Class     authv1beta1.ResourceSliceClass
	Resources map[corev1.ResourceName]string
}

// ResourceSlice forges a ResourceSlice resource.
func ResourceSlice(name, namespace string) *authv1beta1.ResourceSlice {
	return &authv1beta1.ResourceSlice{
		TypeMeta: metav1.TypeMeta{
			APIVersion: authv1beta1.GroupVersion.String(),
			Kind:       authv1beta1.ResourceSliceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// MutateResourceSlice mutates a ResourceSlice resource.
func MutateResourceSlice(resourceSlice *authv1beta1.ResourceSlice, remoteClusterID liqov1beta1.ClusterID,
	opts *ResourceSliceOptions, createVirtualNode bool) error {
	if resourceSlice.Labels == nil {
		resourceSlice.Labels = map[string]string{}
	}
	resourceSlice.Labels[consts.ReplicationRequestedLabel] = consts.ReplicationRequestedLabelValue
	resourceSlice.Labels[consts.ReplicationDestinationLabel] = string(remoteClusterID)
	resourceSlice.Labels[consts.RemoteClusterID] = string(remoteClusterID)

	if createVirtualNode {
		if resourceSlice.Annotations == nil {
			resourceSlice.Annotations = map[string]string{}
		}
		resourceSlice.Annotations[consts.CreateVirtualNodeAnnotation] = "true"
	}

	rl, err := resourceList(opts.Resources)
	if err != nil {
		return err
	}

	resourceSlice.Spec = authv1beta1.ResourceSliceSpec{
		Class:             opts.Class,
		ProviderClusterID: ptr.To(remoteClusterID),
		Resources:         rl,
	}
	return nil
}

func resourceList(resources map[corev1.ResourceName]string) (corev1.ResourceList, error) {
	resourceList := corev1.ResourceList{}
	for name, quantity := range resources {
		if quantity == "" {
			continue
		}

		qnt, err := resource.ParseQuantity(quantity)
		if err != nil {
			return nil, err
		}
		resourceList[name] = qnt
	}

	return resourceList, nil
}
