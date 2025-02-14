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

package tenantnamespace

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
)

type namespaceLister func(ctx context.Context, selector labels.Selector) (ret []*v1.Namespace, err error)
type namespaceGetter func(ctx context.Context, cluster liqov1beta1.ClusterID) (ret *v1.Namespace, err error)

type tenantNamespaceManager struct {
	client                    kubernetes.Interface
	listNamespaces            namespaceLister
	getNamespaceByDefaultName namespaceGetter
	scheme                    *runtime.Scheme
}

// NewManager creates a new TenantNamespaceManager object.
func NewManager(client kubernetes.Interface, scheme *runtime.Scheme) Manager {
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

	getNamespace := func(ctx context.Context, cluster liqov1beta1.ClusterID) (ret *v1.Namespace, err error) {
		defaultNsName := getNameForNamespace(cluster)
		ret, err = client.CoreV1().Namespaces().Get(ctx, defaultNsName, metav1.GetOptions{})

		return
	}

	return &tenantNamespaceManager{
		client:                    client,
		listNamespaces:            listNamespaces,
		getNamespaceByDefaultName: getNamespace,
		scheme:                    scheme,
	}
}

// NewCachedManager creates a new TenantNamespaceManager object, supporting cached retrieval of namespaces for increased efficiency.
func NewCachedManager(ctx context.Context, client kubernetes.Interface, scheme *runtime.Scheme) Manager {
	// Here, we create a new namepace lister, so that it is possible to perform cached get/list operations.
	// The informer factory is configured with an appropriate filter to cache only tenant namespaces.
	req, err := labels.NewRequirement(consts.TenantNamespaceLabel, selection.Exists, []string{})
	utilruntime.Must(err)

	factory := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithTweakListOptions(
		func(lo *metav1.ListOptions) { lo.LabelSelector = labels.NewSelector().Add(*req).String() },
	))
	namespaceLister := factory.Core().V1().Namespaces().Lister()
	listNamespaces := func(_ context.Context, selector labels.Selector) (ret []*v1.Namespace, err error) {
		return namespaceLister.List(selector)
	}

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	return &tenantNamespaceManager{
		client:         client,
		listNamespaces: listNamespaces,
		scheme:         scheme,
	}
}

// CreateNamespace creates a new Tenant Namespace given the clusterid
// This method is idempotent, multiple calls of it will not lead to multiple namespace creations.
func (nm *tenantNamespaceManager) CreateNamespace(ctx context.Context, cluster liqov1beta1.ClusterID) (ns *v1.Namespace, err error) {
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
	ns = nm.ForgeNamespace(cluster, nil)

	ns, err = nm.client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	klog.V(4).Infof("Namespace %v created for the remote cluster %v", ns.Name, cluster)
	return ns, nil
}

// ForgeNamespace returns a Tenant Namespace resource object given name and clusterid.
func (nm *tenantNamespaceManager) ForgeNamespace(cluster liqov1beta1.ClusterID, name *string) *v1.Namespace {
	// If no name is provided use the default one provided by the getNameForNamespace() function
	nsname := getNameForNamespace(cluster)
	if name != nil {
		nsname = *name
	}

	return &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: v1.SchemeGroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: nsname,
			Labels: map[string]string{
				consts.RemoteClusterID:      string(cluster),
				consts.TenantNamespaceLabel: "true",
			},
		},
	}
}

// GetNamespace gets a Tenant Namespace given the clusterid.
func (nm *tenantNamespaceManager) GetNamespace(ctx context.Context, cluster liqov1beta1.ClusterID) (*v1.Namespace, error) {
	req, err := labels.NewRequirement(consts.RemoteClusterID, selection.Equals, []string{string(cluster)})
	utilruntime.Must(err)

	req2, err := labels.NewRequirement(consts.TenantNamespaceLabel, selection.Exists, []string{})
	utilruntime.Must(err)

	labelSelector := labels.NewSelector().Add(*req).Add(*req2)
	namespaces, err := nm.listNamespaces(ctx, labelSelector)
	if kerrors.IsForbidden(err) {
		// Use has not the permissions to access to all the namespaces in the cluster, try to retrieve the default namespace
		ns, err := nm.getNamespaceByDefaultName(ctx, cluster)
		if kerrors.IsNotFound(err) || (ns != nil && !labelSelector.Matches(labels.Set(ns.Labels))) {
			return nil, fmt.Errorf(
				"namespace access is forbidden to current user and no tenant namespace with default name has been created in advance",
			)
		} else if err != nil {
			return nil, err
		}

		namespaces = []*v1.Namespace{ns}
	} else if err != nil {
		klog.Error(err)
		return nil, err
	}

	if nItems := len(namespaces); nItems == 0 {
		err = kerrors.NewNotFound(v1.Resource("Namespace"), string(cluster))
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

// getNameForNamespace given a cluster identity it returns the default name of the tenant namespace for the cluster.
func getNameForNamespace(cluster liqov1beta1.ClusterID) string {
	return fmt.Sprintf("%s-%s", NamePrefix, foreignclusterutils.UniqueName(cluster))
}
