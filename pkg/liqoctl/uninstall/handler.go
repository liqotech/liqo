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
	"errors"
	"fmt"
	"strings"

	"helm.sh/helm/v3/pkg/storage/driver"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualKubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

var liqoGroupVersions = []schema.GroupVersion{
	discoveryv1alpha1.GroupVersion,
	netv1alpha1.GroupVersion,
	offv1alpha1.GroupVersion,
	sharingv1alpha1.GroupVersion,
	virtualKubeletv1alpha1.SchemeGroupVersion,
}

// Options encapsulates the arguments of the uninstall command.
type Options struct {
	*factory.Factory

	Purge bool
}

// Run implements the uninstall command.
func (o *Options) Run(ctx context.Context) error {
	s := o.Printer.StartSpinner("Uninstalling Liqo")

	err := o.HelmClient().UninstallReleaseByName(install.LiqoReleaseName)
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		s.Fail("Error uninstalling Liqo: ", err)
		return err
	}
	s.Success("Liqo uninstalled")

	if o.Purge {
		s = o.Printer.StartSpinner("Purging Liqo CRDs")

		if err = o.purge(ctx); err != nil {
			s.Fail("Error purging CRDs: ", err)
			return err
		}
		s.Success("Liqo CRDs purged")
	}

	return nil
}

func (o *Options) purge(ctx context.Context) error {
	for _, groupVersion := range liqoGroupVersions {
		res, err := o.KubeClient.Discovery().ServerResourcesForGroupVersion(groupVersion.String())
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
			crd := apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: name}}
			err = o.CRClient.Delete(ctx, &crd)
			if client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	}

	return nil
}
