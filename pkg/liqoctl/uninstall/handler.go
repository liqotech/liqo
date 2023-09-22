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

package uninstall

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	helm "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualKubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

var liqoGroupVersions = []schema.GroupVersion{
	discoveryv1alpha1.GroupVersion,
	netv1alpha1.GroupVersion,
	offv1alpha1.GroupVersion,
	sharingv1alpha1.GroupVersion,
	virtualKubeletv1alpha1.SchemeGroupVersion,
	networkingv1alpha1.GroupVersion,
	ipamv1alpha1.GroupVersion,
}

// Options encapsulates the arguments of the uninstall command.
type Options struct {
	*factory.Factory

	Timeout time.Duration
	Purge   bool
}

// Run implements the uninstall command.
func (o *Options) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	s := o.Printer.StartSpinner("Running pre-uninstall checks")
	if err := o.preUninstall(ctx); err != nil {
		s.Fail("Pre-uninstall checks failed: ", output.PrettyErr(err))
		return err
	}
	s.Success("Pre-uninstall checks passed")

	s = o.Printer.StartSpinner("Uninstalling Liqo")
	chartSpec := helm.ChartSpec{ReleaseName: install.LiqoReleaseName, Timeout: o.Timeout}
	err := o.HelmClient().UninstallRelease(&chartSpec)
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		s.Fail("Error uninstalling Liqo: ", output.PrettyErr(err))
		return err
	}

	// Ensure liqo has been correctly uninstalled before continuing, to prevent leaving leftover resources behind.
	if err := o.checkUninstalled(ctx); err != nil {
		s.Fail("Error uninstalling Liqo: ", output.PrettyErr(err))
		return err
	}

	s.Success("Liqo uninstalled")

	if o.Purge {
		s = o.Printer.StartSpinner("Purging Liqo CRDs")

		if err = o.purge(ctx); err != nil {
			s.Fail("Error purging CRDs: ", output.PrettyErr(err))
			return err
		}
		s.Success("Liqo CRDs purged")

		s = o.Printer.StartSpinner("Deleting Liqo namespaces")
		if err = o.deleteLiqoNamespaces(ctx); err != nil {
			s.Fail("Error deleting namespaces: ", output.PrettyErr(err))
			return err
		}
		s.Success("Liqo namespaces deleted")
	}

	return nil
}

func (o *Options) checkUninstalled(ctx context.Context) error {
	var clusterRoles rbacv1.ClusterRoleList
	if err := o.CRClient.List(ctx, &clusterRoles, client.MatchingLabels{"app.kubernetes.io/part-of": install.LiqoReleaseName}); err != nil {
		return fmt.Errorf("failed checking whether cluster-wide resources have been removed: %w", err)
	}

	if len(clusterRoles.Items) > 0 {
		return errors.New("cluster-wide resources are still present - did you specify the right namespace?")
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

func (o *Options) deleteLiqoNamespaces(ctx context.Context) error {
	var nsList corev1.NamespaceList

	// delete tenant namespaces
	// we list them all and then delete them one by one to avoid the error
	// "the server does not allow this method on the requested resource"
	if err := o.CRClient.List(ctx, &nsList, client.MatchingLabels{
		discovery.TenantNamespaceLabel: "true",
	}); err != nil {
		return err
	}
	for i := range nsList.Items {
		ns := &nsList.Items[i]
		if err := o.CRClient.Delete(ctx, ns); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	// delete liqo namespace
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: o.LiqoNamespace,
		},
	}
	if err := o.CRClient.Delete(ctx, &ns); client.IgnoreNotFound(err) != nil {
		return err
	}

	// delete liqo storage namespace
	if err := o.CRClient.List(ctx, &nsList, client.MatchingLabels{
		consts.StorageNamespaceLabel: "true",
	}); err != nil {
		return err
	}
	for i := range nsList.Items {
		ns := &nsList.Items[i]
		if err := o.CRClient.Delete(ctx, ns); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}
