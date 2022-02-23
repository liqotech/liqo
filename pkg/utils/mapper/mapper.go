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

package mapper

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualKubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
)

type LiqoMapper func(c *rest.Config) (meta.RESTMapper, error)

// get the default liqo mapper.
func LiqoMapperProvider(scheme *runtime.Scheme, additionalGroupVersions ...schema.GroupVersion) LiqoMapper {
	mapper := meta.NewDefaultRESTMapper(scheme.PrioritizedVersionsAllGroups())

	return func(c *rest.Config) (meta.RESTMapper, error) {
		dClient, err := discovery.NewDiscoveryClientForConfig(c)
		if err != nil {
			klog.Error(err)
			return nil, err
		}
		if err = addDefaults(dClient, mapper); err != nil {
			klog.Error(err)
			return nil, err
		}

		for _, gv := range additionalGroupVersions {
			if err = addGroup(dClient, gv, mapper); err != nil {
				klog.Error(err)
				return nil, err
			}
		}

		return mapper, nil
	}
}

// add most used groups to the mapper, this includes all Liqo groups with core/v1, apps/v1 and rbac/v1.
func addDefaults(dClient *discovery.DiscoveryClient, mapper *meta.DefaultRESTMapper) error {
	var err error

	// Liqo groups
	if err = addGroup(dClient, discoveryv1alpha1.GroupVersion, mapper); err != nil {
		return err
	}
	if err = addGroup(dClient, netv1alpha1.GroupVersion, mapper); err != nil {
		return err
	}
	if err = addGroup(dClient, sharingv1alpha1.GroupVersion, mapper); err != nil {
		return err
	}
	if err = addGroup(dClient, virtualKubeletv1alpha1.SchemeGroupVersion, mapper); err != nil {
		return err
	}
	if err = addGroup(dClient, offv1alpha1.GroupVersion, mapper); err != nil {
		return err
	}

	// Kubernetes groups
	if err = addGroup(dClient, corev1.SchemeGroupVersion, mapper); err != nil {
		return err
	}
	if err = addGroup(dClient, appsv1.SchemeGroupVersion, mapper); err != nil {
		return err
	}
	if err = addGroup(dClient, rbacv1.SchemeGroupVersion, mapper); err != nil {
		return err
	}
	return addGroup(dClient, storagev1.SchemeGroupVersion, mapper)
}

// add all the resources in the specified groupVersion to the mapper.
func addGroup(dClient *discovery.DiscoveryClient, groupVersion schema.GroupVersion, mapper *meta.DefaultRESTMapper) error {
	res, err := dClient.ServerResourcesForGroupVersion(groupVersion.String())
	if err != nil {
		klog.Error(err)
		return err
	}
	for _, apiRes := range res.APIResources {
		var scope meta.RESTScope
		if apiRes.Namespaced {
			scope = meta.RESTScopeNamespace
		} else {
			scope = meta.RESTScopeRoot
		}
		mapper.Add(schema.GroupVersionKind{
			Group:   groupVersion.Group,
			Version: groupVersion.Version,
			Kind:    apiRes.Kind,
		}, scope)
	}
	return nil
}
