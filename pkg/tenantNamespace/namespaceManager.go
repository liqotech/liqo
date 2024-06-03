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

package tenantnamespace

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
)

type namespaceLister func(ctx context.Context, selector labels.Selector) (ret []*v1.Namespace, err error)

type tenantNamespaceManager struct {
	client         kubernetes.Interface
	listNamespaces namespaceLister
}

// NewManager creates a new TenantNamespaceManager object.
func NewManager(client kubernetes.Interface) Manager {
	listNamespaces := func(ctx context.Context, selector labels.Selector) (ret []*v1.Namespace, err error) {
		ns, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			return nil, err
		}

		nsref := make([]*v1.Namespace, len(ns.Items))
		for i := range ns.Items {
			nsref[i] = &ns.Items[i]
		}
		return nsref, nil
	}

	return &tenantNamespaceManager{
		client:         client,
		listNamespaces: listNamespaces,
	}
}

// NewCachedManager creates a new TenantNamespaceManager object, supporting cached retrieval of namespaces for increased efficiency.
func NewCachedManager(ctx context.Context, client kubernetes.Interface) Manager {
	// Here, we create a new namepace lister, so that it is possible to perform cached get/list operations.
	// The informer factory is configured with an appropriate filter to cache only tenant namespaces.
	req, err := labels.NewRequirement(discovery.TenantNamespaceLabel, selection.Exists, []string{})
	utilruntime.Must(err)

	factory := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithTweakListOptions(
		func(lo *metav1.ListOptions) { lo.LabelSelector = labels.NewSelector().Add(*req).String() },
	))
	namespaceLister := factory.Core().V1().Namespaces().Lister()
	listNamespaces := func(ctx context.Context, selector labels.Selector) (ret []*v1.Namespace, err error) {
		return namespaceLister.List(selector)
	}

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	return &tenantNamespaceManager{
		client:         client,
		listNamespaces: listNamespaces,
	}
}

// CreateNamespace creates a new Tenant Namespace given the clusterid
// This method is idempotent, multiple calls of it will not lead to multiple namespace creations.
func (nm *tenantNamespaceManager) CreateNamespace(ctx context.Context, cluster discoveryv1alpha1.ClusterID) (ns *v1.Namespace, err error) {
	// Let immediately check if the namespace already exists, since this might be cached and thus fast
	if ns, err = nm.GetNamespace(ctx, cluster); err == nil {
		return ns, nil
	} else if !kerrors.IsNotFound(err) {
		klog.Error(err)
		return nil, err
	}

	// The namespace was not found, hence create it. Since GetNamespace was cached, a race condition might occur,
	// and the creation might fail because the namespace already exists. Still, in this case the controller will
	// exit with an error, and retry during the next iteration.
	ns = &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: GetNameForNamespace(cluster),
			Labels: map[string]string{
				discovery.ClusterIDLabel:       string(cluster),
				discovery.TenantNamespaceLabel: "true",
			},
		},
	}

	ns, err = nm.client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	klog.V(4).Infof("Namespace %v created for the remote cluster %v", ns.Name, cluster)
	return ns, nil
}

// GetNamespace gets a Tenant Namespace given the clusterid.
func (nm *tenantNamespaceManager) GetNamespace(ctx context.Context, cluster discoveryv1alpha1.ClusterID) (*v1.Namespace, error) {
	req, err := labels.NewRequirement(discovery.ClusterIDLabel, selection.Equals, []string{string(cluster)})
	utilruntime.Must(err)

	namespaces, err := nm.listNamespaces(ctx, labels.NewSelector().Add(*req))
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	if nItems := len(namespaces); nItems == 0 {
		err = kerrors.NewNotFound(v1.Resource("Namespace"), GetNameForNamespace(cluster))
		// do not log it always, since it is also used in the preliminary stage of the create method
		klog.V(4).Info(err)
		return nil, err
	} else if nItems > 1 {
		err = fmt.Errorf("multiple tenant namespaces found for clusterid %v", cluster)
		klog.Error(err)
		return nil, err
	}
	return namespaces[0].DeepCopy(), nil
}

// GetNameForNamespace given a cluster identity it returns the name of the tenant namespace for the cluster.
func GetNameForNamespace(cluster discoveryv1alpha1.ClusterID) string {
	return fmt.Sprintf("%s-%s", NamePrefix, foreignclusterutils.UniqueName(cluster))
}
