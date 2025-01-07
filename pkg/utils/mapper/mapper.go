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

package mapper

import (
	"errors"
	"net/http"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	adminssionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

// LiqoMapper is a function that returns a RESTMapper for the resources used by Liqo.
type LiqoMapper func(c *rest.Config, httpClient *http.Client) (meta.RESTMapper, error)

// LiqoMapperProvider returns the default liqo mapper.
func LiqoMapperProvider(scheme *runtime.Scheme, additionalGroupVersions ...schema.GroupVersion) LiqoMapper {
	mapper := meta.NewDefaultRESTMapper(scheme.PrioritizedVersionsAllGroups())

	return func(c *rest.Config, _ *http.Client) (meta.RESTMapper, error) {
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
			if err = addGroup(dClient, gv, mapper, GroupRequired); err != nil {
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
	if err = addGroup(dClient, liqov1beta1.GroupVersion, mapper, GroupRequired); err != nil {
		return err
	}
	if err = addGroup(dClient, offloadingv1beta1.SchemeGroupVersion, mapper, GroupRequired); err != nil {
		return err
	}
	if err = addGroup(dClient, ipamv1alpha1.SchemeGroupVersion, mapper, GroupRequired); err != nil {
		return err
	}
	if err = addGroup(dClient, networkingv1beta1.GroupVersion, mapper, GroupRequired); err != nil {
		return err
	}
	if err = addGroup(dClient, authv1beta1.GroupVersion, mapper, GroupRequired); err != nil {
		return err
	}

	// Kubernetes groups
	if err = addGroup(dClient, corev1.SchemeGroupVersion, mapper, GroupRequired); err != nil {
		return err
	}
	if err = addGroup(dClient, appsv1.SchemeGroupVersion, mapper, GroupRequired); err != nil {
		return err
	}
	if err = addGroup(dClient, rbacv1.SchemeGroupVersion, mapper, GroupRequired); err != nil {
		return err
	}
	if err = addGroup(dClient, discoveryv1.SchemeGroupVersion, mapper, GroupRequired); err != nil {
		return err
	}
	if err = addGroup(dClient, storagev1.SchemeGroupVersion, mapper, GroupRequired); err != nil {
		return err
	}
	if err = addGroup(dClient, adminssionregistrationv1.SchemeGroupVersion, mapper, GroupRequired); err != nil {
		return err
	}

	// Prometheus operator group
	if err = addGroup(dClient, monitoringv1.SchemeGroupVersion, mapper, GroupOptional); err != nil {
		return err
	}

	mapper.Add(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "List",
	}, meta.RESTScopeRoot)
	return nil
}

const (
	// GroupRequired is used to specify that a group is required by the mapper.
	GroupRequired = true
	// GroupOptional is used to specify that a group is optional for the mapper.
	GroupOptional = false
)

// add all the resources in the specified groupVersion to the mapper.
func addGroup(dClient *discovery.DiscoveryClient, groupVersion schema.GroupVersion,
	mapper *meta.DefaultRESTMapper, required bool) error {
	res, err := dClient.ServerResourcesForGroupVersion(groupVersion.String())
	var dErr *apierrors.StatusError
	switch {
	case errors.As(err, &dErr) && !required:
		// ignore error, and do not add the group to the mapper, the CRD is not available.
		return nil
	case err != nil:
		klog.Error(err)
		return err
	}
	for i := range res.APIResources {
		apiRes := &res.APIResources[i]
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
		mapper.Add(schema.GroupVersionKind{
			Group:   groupVersion.Group,
			Version: groupVersion.Version,
			Kind:    apiRes.Kind + "List",
		}, scope)
	}
	return nil
}
