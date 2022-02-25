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
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

type tenantNamespaceManager struct {
	client          kubernetes.Interface
	namespaceLister corev1listers.NamespaceLister
}

// NewTenantNamespaceManager creates a new TenantNamespaceManager object.
func NewTenantNamespaceManager(client kubernetes.Interface) Manager {
	// TODO: the context should be propagated from the caller. It is currently set here to avoid
	// modifying all callers, since most to not even have a proper context themselves.
	ctx := context.Background()

	// Here, we create a new namepace lister, so that it is possible to perform cached get/list operations.
	// The informer factory is configured with an appropriate filter to cache only tenant namespaces.
	req, err := labels.NewRequirement(discovery.TenantNamespaceLabel, selection.Exists, []string{})
	utilruntime.Must(err)

	factory := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithTweakListOptions(
		func(lo *metav1.ListOptions) { lo.LabelSelector = labels.NewSelector().Add(*req).String() },
	))
	namespaceLister := factory.Core().V1().Namespaces().Lister()

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	return &tenantNamespaceManager{
		client:          client,
		namespaceLister: namespaceLister,
	}
}

// CreateNamespace creates a new Tenant Namespace given the clusterid
// This method is idempotent, multiple calls of it will not lead to multiple namespace creations.
func (nm *tenantNamespaceManager) CreateNamespace(cluster discoveryv1alpha1.ClusterIdentity) (ns *v1.Namespace, err error) {
	// Let immediately check if the namespace already exists, since this is operation cached and thus fast
	if ns, err = nm.GetNamespace(cluster); err == nil {
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
				discovery.ClusterIDLabel:       cluster.ClusterID,
				discovery.TenantNamespaceLabel: "true",
			},
		},
	}

	ns, err = nm.client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	klog.V(4).Infof("Namespace %v created for the remote cluster %v", ns.Name, cluster.ClusterName)
	return ns, nil
}

// GetNamespace gets a Tenant Namespace given the clusterid.
func (nm *tenantNamespaceManager) GetNamespace(cluster discoveryv1alpha1.ClusterIdentity) (*v1.Namespace, error) {
	req, err := labels.NewRequirement(discovery.ClusterIDLabel, selection.Equals, []string{cluster.ClusterID})
	utilruntime.Must(err)

	namespaces, err := nm.namespaceLister.List(labels.NewSelector().Add(*req))
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
		err = fmt.Errorf("multiple tenant namespaces found for clusterid %v", cluster.ClusterName)
		klog.Error(err)
		return nil, err
	}
	return namespaces[0].DeepCopy(), nil
}

// GetNameForNamespace given a cluster identity it returns the name of the tenant namespace for the cluster.
func GetNameForNamespace(cluster discoveryv1alpha1.ClusterIdentity) string {
	return fmt.Sprintf("liqo-tenant-%s", foreignclusterutils.UniqueName(&cluster))
}
