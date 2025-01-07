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

package remoteresourceslicecontroller

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	argutils "github.com/liqotech/liqo/pkg/utils/args"
)

// SliceStatusOptions contains the options to configure the status of a remote resource slice.
type SliceStatusOptions struct {
	EnableStorage             bool
	LocalRealStorageClassName string
	IngressClasses            argutils.ClassNameList
	LoadBalancerClasses       argutils.ClassNameList
	ClusterLabels             map[string]string
	DefaultResourceQuantity   corev1.ResourceList
}

func getIngressClasses(opts *SliceStatusOptions) []liqov1beta1.IngressType {
	if opts == nil {
		return []liqov1beta1.IngressType{}
	}

	ingressClasses := make([]liqov1beta1.IngressType, len(opts.IngressClasses.Classes))
	for i := range opts.IngressClasses.Classes {
		ingressClasses[i].IngressClassName = opts.IngressClasses.Classes[i].Name
		ingressClasses[i].Default = opts.IngressClasses.Classes[i].IsDefault
	}
	return ingressClasses
}

func getLoadBalancerClasses(opts *SliceStatusOptions) []liqov1beta1.LoadBalancerType {
	if opts == nil {
		return []liqov1beta1.LoadBalancerType{}
	}

	loadBalancerClasses := make([]liqov1beta1.LoadBalancerType, len(opts.LoadBalancerClasses.Classes))
	for i := range opts.LoadBalancerClasses.Classes {
		loadBalancerClasses[i].LoadBalancerClassName = opts.LoadBalancerClasses.Classes[i].Name
		loadBalancerClasses[i].Default = opts.LoadBalancerClasses.Classes[i].IsDefault
	}
	return loadBalancerClasses
}

func getStorageClasses(ctx context.Context, cl client.Client, opts *SliceStatusOptions) ([]liqov1beta1.StorageType, error) {
	if opts == nil || !opts.EnableStorage {
		return []liqov1beta1.StorageType{}, nil
	}

	storageClassList := &storagev1.StorageClassList{}
	err := cl.List(ctx, storageClassList)
	if err != nil {
		return nil, err
	}

	storageTypes := make([]liqov1beta1.StorageType, len(storageClassList.Items))
	for i := range storageClassList.Items {
		class := &storageClassList.Items[i]
		storageTypes[i].StorageClassName = class.GetName()

		// set the storage class as default if:
		// 1. it is the real storage class of the local cluster
		// 2. no local real storage class is set and it is the cluster default storage class
		if opts.LocalRealStorageClassName == "" {
			if val, ok := class.Annotations["storageclass.kubernetes.io/is-default-class"]; ok && val == "true" {
				storageTypes[i].Default = true
			}
		} else if class.GetName() == opts.LocalRealStorageClassName {
			storageTypes[i].Default = true
		}
	}

	// sort the storage classes by name to have a deterministic order
	sort.Slice(storageTypes, func(i, j int) bool {
		return storageTypes[i].StorageClassName < storageTypes[j].StorageClassName
	})

	return storageTypes, nil
}

func getNodeLabels(opts *SliceStatusOptions) map[string]string {
	if opts == nil {
		return map[string]string{}
	}
	return opts.ClusterLabels
}
