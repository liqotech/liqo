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

package uninstall

import (
	"context"
	"fmt"
	"strings"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualKubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
)

var liqoGroupVersions = []schema.GroupVersion{
	discoveryv1alpha1.GroupVersion,
	netv1alpha1.GroupVersion,
	offv1alpha1.GroupVersion,
	sharingv1alpha1.GroupVersion,
	virtualKubeletv1alpha1.SchemeGroupVersion,
}

func purge(ctx context.Context, config *rest.Config) error {
	dClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}

	clientSet, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return err
	}

	if err = removeGroupVersions(ctx, dClient, clientSet, liqoGroupVersions); err != nil {
		return err
	}

	return nil
}

func removeGroupVersions(ctx context.Context,
	dClient discovery.DiscoveryInterface, clientSet apiextensionsclientset.Interface,
	groupVersions []schema.GroupVersion) error {
	for _, groupVersion := range groupVersions {
		res, err := dClient.ServerResourcesForGroupVersion(groupVersion.String())
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		for i := range res.APIResources {
			apiRes := &res.APIResources[i]
			if strings.Contains(apiRes.Name, "/") {
				// skip subresources
				continue
			}
			name := fmt.Sprintf("%s.%s", apiRes.Name, groupVersion.Group)
			err = clientSet.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, name, metav1.DeleteOptions{})
			if client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	}
	return nil
}
