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

package foreigncluster

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
)

// GetForeignClusterByID returns a ForeignCluster CR retrieving it by its clusterID.
func GetForeignClusterByID(ctx context.Context, cl client.Client, clusterID string) (*discoveryv1alpha1.ForeignCluster, error) {
	lSelector := labels.SelectorFromSet(labels.Set{
		discovery.ClusterIDLabel: clusterID,
	})
	// get the foreign cluster by clusterID label
	foreignClusterList := discoveryv1alpha1.ForeignClusterList{}
	if err := cl.List(ctx, &foreignClusterList, &client.ListOptions{
		LabelSelector: lSelector,
	}); err != nil {
		return nil, err
	}

	switch len(foreignClusterList.Items) {
	case 0:
		return nil, kerrors.NewNotFound(discoveryv1alpha1.ForeignClusterGroupResource, fmt.Sprintf("foreign cluster with ID %s", clusterID))
	case 1:
		return &foreignClusterList.Items[0], nil
	default:
		return GetOlderForeignCluster(&foreignClusterList), nil
	}
}

// GetOlderForeignCluster returns the ForeignCluster from the list with the older creationTimestamp.
func GetOlderForeignCluster(
	foreignClusterList *discoveryv1alpha1.ForeignClusterList) (foreignCluster *discoveryv1alpha1.ForeignCluster) {
	var olderTime *metav1.Time
	for i := range foreignClusterList.Items {
		fc := &foreignClusterList.Items[i]
		if olderTime.IsZero() || fc.CreationTimestamp.Before(olderTime) {
			olderTime = &fc.CreationTimestamp
			foreignCluster = fc
		}
	}
	return foreignCluster
}

// getAuthAddress retrieves the external address where the Authentication Service is reachable from the external world.
func getAuthAddress(ctx context.Context, cl client.Client, svc *corev1.Service) (string, error) {
	if overrideAddress, ok := svc.Annotations[liqoconst.OverrideAddressAnnotation]; ok {
		overrideAddress = strings.TrimPrefix(overrideAddress, "https://")
		overrideAddress = strings.TrimPrefix(overrideAddress, "http://")
		overrideAddress = strings.TrimSuffix(overrideAddress, "/")
		return overrideAddress, nil
	}

	// if the service is exposed as LoadBalancer
	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		// get the IP from the LoadBalancer service
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			// the service has no external IPs
			err := errors.New("no valid external IP for LoadBalancer Service")
			klog.Error(err)
			return "", err
		}
		lbIngress := svc.Status.LoadBalancer.Ingress[0]
		// return the external service IP
		if hostname := lbIngress.Hostname; hostname != "" {
			return hostname, nil
		} else if ip := lbIngress.IP; ip != "" {
			return ip, nil
		} else {
			// the service has no external IPs
			err := errors.New("no valid external IP for LoadBalancer Service")
			klog.Error(err)
			return "", err
		}
	}

	// only physical nodes
	//
	// we need to get an address from a physical node, if we have established peerings in the past with other clusters,
	// we may have some virtual nodes in our cluster. Since their IPs will not be reachable from other clusters, we cannot use them
	// as address for a local NodePort Service
	req, err := labels.NewRequirement(liqoconst.TypeLabel, selection.NotIn, []string{liqoconst.TypeNode})
	utilruntime.Must(err)

	// get the IP from the Nodes, to be used with NodePort services
	nodes := corev1.NodeList{}
	if err := cl.List(ctx, &nodes, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(*req)}); err != nil {
		klog.Error(err)
		return "", err
	}

	if len(nodes.Items) == 0 {
		// there are no node is the cluster, we cannot get the address on any of them
		err = kerrors.NewNotFound(corev1.Resource("nodes"), "")
		klog.Error(err)
		return "", err
	}

	node := nodes.Items[0]
	return discovery.GetAddress(&node)

	// when an error occurs, it means that we were not able to get an address in any of the previous cases:
	// 1. no overwrite variable is set
	// 2. the service is not of type LoadBalancer
	// 3. there are no nodes in the cluster to get the IP for a NodePort service
}

// getAuthPort retrieves the external port where the Authentication Service is reachable from the external world.
func getAuthPort(svc *corev1.Service) (string, error) {
	if overridePort, ok := svc.Annotations[liqoconst.OverridePortAnnotation]; ok {
		return overridePort, nil
	}

	if len(svc.Spec.Ports) == 0 {
		// the service has no available port, we cannot get it
		err := kerrors.NewNotFound(corev1.Resource(corev1.ResourceServices.String()), liqoconst.AuthServiceName)
		klog.Error(err)
		return "", err
	}

	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		// return the LoadBalancer service external port
		return fmt.Sprintf("%v", svc.Spec.Ports[0].Port), nil
	}
	if svc.Spec.Type == corev1.ServiceTypeNodePort {
		// return the NodePort service port
		return fmt.Sprintf("%v", svc.Spec.Ports[0].NodePort), nil
	}
	// other service types. When we are using an Ingress we should not reach this code, because of the environment variable
	return "",
		fmt.Errorf(
			"you cannot expose the Auth Service with a %v Service. If you are using an Ingress, probably, there are configuration issues",
			svc.Spec.Type)
}

// GetHomeAuthURL retrieves the auth service endpoint by inspecting the cluster. It returns an empty string and an error if it does not succeed.
func GetHomeAuthURL(ctx context.Context, cl client.Client, liqoNamespace string) (string, error) {
	// get the authentication service
	var svc corev1.Service
	ref := types.NamespacedName{Name: liqoconst.AuthServiceName, Namespace: liqoNamespace}
	if err := cl.Get(ctx, ref, &svc); err != nil {
		klog.Error(err)
		return "", err
	}

	address, err := getAuthAddress(ctx, cl, &svc)
	if err != nil {
		return "", err
	}

	port, err := getAuthPort(&svc)
	if err != nil {
		return "", err
	}
	if port != "443" {
		return fmt.Sprintf("https://%s:%s", address, port), nil
	}
	return fmt.Sprintf("https://%s", address), nil
}
