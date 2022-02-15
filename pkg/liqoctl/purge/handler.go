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

package purge

import (
	"context"
	"fmt"
	"time"

	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/common"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
)

func initClusterHandler(color pterm.Color, number int, config string) (*clusterHandler, error) {
	printer := common.NewPrinter("", color)

	s, err := printer.Spinner.Start(fmt.Sprintf("Loading configuration for cluster %d", number))
	utilruntime.Must(err)

	cl, err := getClient(config)
	if err != nil {
		s.Fail(fmt.Sprintf("Failed to load configuration for cluster %d: %v", number, err))
		return nil, err
	}

	nativeCl, err := getKClient(config)
	if err != nil {
		s.Fail(fmt.Sprintf("Failed to load configuration for cluster %d: %v", number, err))
		return nil, err
	}
	s.Success(fmt.Sprintf("Loaded configuration for cluster %d", number))

	return &clusterHandler{
		color:                color,
		number:               number,
		printer:              printer,
		cl:                   cl,
		nativeCl:             nativeCl,
		hasFullClusterAccess: true,
	}, nil
}

// HandlePurgeCommand implements the "purge" command.
func HandlePurgeCommand(ctx context.Context, args *Args) error {
	handler1, err := initClusterHandler(common.Cluster1Color, 1, args.Config1)
	if err != nil {
		return err
	}

	h := handler{
		handler1: handler1,
	}

	switch {
	case args.Config2 == "" && args.RemoteCluster == "":
		err = fmt.Errorf("you must specify a remote cluster or a config file")
		h.handler1.printer.Error.Println(err)
		return err
	case args.Config2 != "" && args.RemoteCluster != "":
		err = fmt.Errorf("you must specify only one of the following: a config file or a remote cluster")
		h.handler1.printer.Error.Println(err)
		return err
	case args.Config2 != "":
		h.handler2, err = initClusterHandler(common.Cluster2Color, 2, args.Config2)
		if err != nil {
			return err
		}
		return h.handlePurgeCommand(ctx, args)
	case args.RemoteCluster != "":
		printer2 := common.NewPrinter("", common.Cluster2Color)
		h.handler2 = &clusterHandler{printer: printer2, cl: h.handler1.cl, number: 2, color: common.Cluster2Color}
		return h.handlePurgeCommand(ctx, args)
	default:
		err = fmt.Errorf("this should never happen")
		h.handler1.printer.Error.Println(err)
		return err
	}
}

func (h *clusterHandler) fetchClusterIdentity(ctx context.Context, args *Args) error {
	s, err := h.printer.Spinner.Start(fmt.Sprintf("Retrieving cluster identity for cluster %d", h.number))
	utilruntime.Must(err)

	switch {
	case h.hasFullClusterAccess:
		h.localClusterIdentity, err = utils.GetClusterIdentityWithControllerClient(ctx, h.cl, installutils.LiqoNamespace)
		if err != nil {
			s.Fail(fmt.Sprintf("Failed to load cluster identity for cluster %d: %v", h.number, err))
			return err
		}
	default:
		// it uses the client for the other cluster
		var fc discoveryv1alpha1.ForeignCluster
		if err = h.cl.Get(ctx, client.ObjectKey{
			Name: args.RemoteCluster,
		}, &fc); err != nil {
			s.Fail(fmt.Sprintf("Failed to load cluster identity for cluster %d: %v", h.number, err))
			return err
		}
		h.localClusterIdentity = fc.Spec.ClusterIdentity
	}

	s.Success(fmt.Sprintf("Retrieved cluster identity for cluster %s (%s)",
		h.localClusterIdentity.ClusterName, h.localClusterIdentity.ClusterID))

	h.printer = common.NewPrinter(h.localClusterIdentity.ClusterName, h.color)
	return nil
}

func (h *handler) handlePurgeCommand(ctx context.Context, args *Args) error {
	err := h.handler1.fetchClusterIdentity(ctx, args)
	if err != nil {
		return err
	}
	err = h.handler2.fetchClusterIdentity(ctx, args)
	if err != nil {
		return err
	}

	h.handler1.remoteClusterIdentity = h.handler2.localClusterIdentity
	h.handler2.remoteClusterIdentity = h.handler1.localClusterIdentity

	err = h.handler1.enforceUnpeer(ctx)
	if err != nil {
		return err
	}
	err = h.handler2.enforceUnpeer(ctx)
	if err != nil {
		return err
	}

	h.handler1.waitForUnpeer(ctx, args.Timeout)
	h.handler2.waitForUnpeer(ctx, args.Timeout)

	err = h.handler1.deleteForeignCluster(ctx)
	if err != nil {
		return err
	}
	err = h.handler2.deleteForeignCluster(ctx)
	if err != nil {
		return err
	}

	err = h.handler1.deleteTenantNamespace(ctx)
	if err != nil {
		return err
	}
	err = h.handler2.deleteTenantNamespace(ctx)
	if err != nil {
		return err
	}

	err = h.handler1.deleteNode(ctx)
	if err != nil {
		return err
	}
	return h.handler2.deleteNode(ctx)
}

func (h *clusterHandler) deleteForeignCluster(ctx context.Context) error {
	if !h.hasFullClusterAccess {
		return nil
	}

	s, err := h.printer.Spinner.Start(fmt.Sprintf("Deleting foreign cluster %s (%s)",
		h.remoteClusterIdentity.ClusterName, h.remoteClusterIdentity.ClusterID))
	utilruntime.Must(err)

	forceDelete := func() error {
		fc, err := foreignclusterutils.GetForeignClusterByID(ctx, h.cl, h.remoteClusterIdentity.ClusterID)
		switch {
		case client.IgnoreNotFound(err) != nil:
			return err
		case err != nil:
			// is not found error
			return nil
		}

		if err := client.IgnoreNotFound(h.cl.Delete(ctx, fc)); err != nil {
			return err
		}

		if fc != nil {
			fc.Finalizers = []string{}
			if err := client.IgnoreNotFound(h.cl.Update(ctx, fc)); err != nil {
				return err
			}
		}

		return nil
	}

	if err = retry.RetryOnConflict(retry.DefaultBackoff, forceDelete); err != nil {
		s.Fail(fmt.Sprintf("Failed to delete foreign cluster %s (%s): %v",
			h.remoteClusterIdentity.ClusterName, h.remoteClusterIdentity.ClusterID, err))
		return err
	}

	s.Success(fmt.Sprintf("Deleted foreign cluster %s (%s)",
		h.remoteClusterIdentity.ClusterName, h.remoteClusterIdentity.ClusterID))
	return nil
}

func (h *clusterHandler) waitForUnpeer(ctx context.Context, timeout time.Duration) {
	if !h.hasFullClusterAccess || timeout == 0 {
		return
	}

	s, err := h.printer.Spinner.Start("Waiting for unpeer to complete")
	utilruntime.Must(err)

	fc, err := foreignclusterutils.GetForeignClusterByID(ctx, h.cl, h.remoteClusterIdentity.ClusterID)
	if err != nil {
		s.Warning(fmt.Sprintf("Failed to wait for unpeer to complete: %v", err))
		return
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()

	for {
		select {
		case <-ctxTimeout.Done():
			s.Warning("Failed to wait for unpeer to complete: unpeer timed out")
			return
		case <-ticker.C:
			err := h.cl.Get(ctxTimeout, client.ObjectKey{Name: fc.Name}, fc)
			if err != nil {
				if apierrors.IsNotFound(err) {
					s.Success("Unpeer completed")
					return
				}
				s.Warning(fmt.Sprintf("Failed to wait for unpeer to complete: %v", err))
				return
			}

			peeringPhase := foreignclusterutils.GetPeeringPhase(fc)
			if peeringPhase == consts.PeeringPhaseNone || peeringPhase == consts.PeeringPhaseAuthenticated {
				s.Success("Unpeer completed")
				return
			}
		}
	}
}

func (h *clusterHandler) deleteTenantNamespace(ctx context.Context) error {
	if !h.hasFullClusterAccess {
		return nil
	}

	s, err := h.printer.Spinner.Start("Deleting tenant namespace")
	utilruntime.Must(err)

	tenantNamespaceManager := tenantnamespace.NewTenantNamespaceManager(h.nativeCl)
	tenantNamespace, err := tenantNamespaceManager.GetNamespace(h.remoteClusterIdentity)
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("Failed to delete tenant namespace: %v", err))
		return err
	} else if apierrors.IsNotFound(err) {
		s.Success("Tenant namespace already deleted")
		return nil
	}
	namespace := tenantNamespace.GetName()

	if err := forceDeleteNamespaceMaps(ctx, h.cl, namespace, s); err != nil {
		s.Fail(fmt.Sprintf("Failed to delete namespace maps: %v", err))
		return err
	}

	if err := forceDeleteNetworkConfigs(ctx, h.cl, namespace, s); err != nil {
		s.Fail(fmt.Sprintf("Failed to delete network configs: %v", err))
		return err
	}

	if err := forceDeleteResourceRequests(ctx, h.cl, namespace, s); err != nil {
		s.Fail(fmt.Sprintf("Failed to delete resource requests: %v", err))
		return err
	}

	if err := forceDeleteResourceOffers(ctx, h.cl, namespace, s); err != nil {
		s.Fail(fmt.Sprintf("Failed to delete resource offers: %v", err))
		return err
	}

	if err := client.IgnoreNotFound(h.cl.Delete(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	})); err != nil {
		s.Fail(fmt.Sprintf("Failed to delete tenant namespace %s: %v", namespace, err))
		return err
	}

	s.Success(fmt.Sprintf("Deleted tenant namespace %s", namespace))
	return nil
}

func (h *clusterHandler) deleteNode(ctx context.Context) error {
	if !h.hasFullClusterAccess {
		return nil
	}

	s, err := h.printer.Spinner.Start("Deleting virtual node")
	utilruntime.Must(err)

	nodeName := virtualKubelet.VirtualNodeName(h.remoteClusterIdentity)
	if err = h.cl.Delete(ctx, &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}); err != nil {
		if apierrors.IsNotFound(err) {
			s.Success("Deleted virtual node")
			return nil
		}
		s.Fail(fmt.Sprintf("Failed to delete virtual node: %v", err))
		return err
	}

	if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var node corev1.Node
		if err := h.cl.Get(ctx, client.ObjectKey{Name: nodeName}, &node); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		node.SetFinalizers([]string{})
		return client.IgnoreNotFound(h.cl.Update(ctx, &node))
	}); err != nil {
		s.Fail(fmt.Sprintf("Failed to remove finalizers from node %q: %v", nodeName, err))
		return err
	}

	var nodeList corev1.NodeList
	if err := h.cl.List(ctx, &nodeList, client.MatchingLabels{
		consts.RemoteClusterID: h.remoteClusterIdentity.ClusterID}); err != nil {
		s.Fail(fmt.Sprintf("Failed to list nodes: %v", err))
		return err
	}

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		node.Finalizers = []string{}
		if err := client.IgnoreNotFound(h.cl.Update(ctx, node)); err != nil {
			s.WarningPrinter.Println(fmt.Sprintf("Failed to remove finalizers from node %s: %v", node.Name, err))
			continue
		}
	}

	s.Success("Deleted virtual node")
	return nil
}

func pointerList[T any](src []T) []*T {
	dst := make([]*T, len(src))
	for i := range src {
		dst[i] = &src[i]
	}
	return dst
}

func forceDeleteNetworkConfigs(ctx context.Context, cl client.Client, namespace string, s *pterm.SpinnerPrinter) error {
	return forceDelete(ctx, cl, namespace, s, &netv1alpha1.NetworkConfig{}, &netv1alpha1.NetworkConfigList{},
		func(list *netv1alpha1.NetworkConfigList) []*netv1alpha1.NetworkConfig { return pointerList(list.Items) })
}

func forceDeleteResourceRequests(ctx context.Context, cl client.Client, namespace string, s *pterm.SpinnerPrinter) error {
	return forceDelete(ctx, cl, namespace, s, &discoveryv1alpha1.ResourceRequest{}, &discoveryv1alpha1.ResourceRequestList{},
		func(list *discoveryv1alpha1.ResourceRequestList) []*discoveryv1alpha1.ResourceRequest {
			return pointerList(list.Items)
		})
}

func forceDeleteResourceOffers(ctx context.Context, cl client.Client, namespace string, s *pterm.SpinnerPrinter) error {
	return forceDelete(ctx, cl, namespace, s, &sharingv1alpha1.ResourceOffer{}, &sharingv1alpha1.ResourceOfferList{},
		func(list *sharingv1alpha1.ResourceOfferList) []*sharingv1alpha1.ResourceOffer {
			return pointerList(list.Items)
		})
}

func forceDeleteNamespaceMaps(ctx context.Context, cl client.Client, namespace string, s *pterm.SpinnerPrinter) error {
	return forceDelete(ctx, cl, namespace, s, &virtualkubeletv1alpha1.NamespaceMap{}, &virtualkubeletv1alpha1.NamespaceMapList{},
		func(list *virtualkubeletv1alpha1.NamespaceMapList) []*virtualkubeletv1alpha1.NamespaceMap {
			return pointerList(list.Items)
		})
}

func forceDelete[T client.Object, U client.ObjectList](ctx context.Context, cl client.Client,
	namespace string, s *pterm.SpinnerPrinter, obj T, objList U, objListToObjArray func(U) []T) error {
	if err := cl.DeleteAllOf(ctx, obj, client.InNamespace(namespace)); err != nil {
		return err
	}

	if err := cl.List(ctx, objList, client.InNamespace(namespace)); err != nil {
		return err
	}

	for _, obj := range objListToObjArray(objList) {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := cl.Get(ctx,
				client.ObjectKey{
					Namespace: obj.GetNamespace(), // nolint:typecheck // golangci is not able to understand it at the moment https://github.com/golangci/golangci-lint/issues/2649
					Name:      obj.GetName()},     // nolint:typecheck // golangci is not able to understand it at the moment https://github.com/golangci/golangci-lint/issues/2649
				obj); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}

			obj.SetFinalizers(nil) // nolint:typecheck // golangci is not able to understand it at the moment https://github.com/golangci/golangci-lint/issues/2649
			return client.IgnoreNotFound(cl.Update(ctx, obj))
		}); err != nil {
			return fmt.Errorf("failed to remove finalizers from %s: %w", obj.GetName(), err)
		}
	}

	return nil
}

func (h *clusterHandler) enforceUnpeer(ctx context.Context) error {
	if !h.hasFullClusterAccess {
		return nil
	}

	s, err := h.printer.Spinner.Start(fmt.Sprintf("Unpeering cluster %s (%s)",
		h.remoteClusterIdentity.ClusterName, h.remoteClusterIdentity.ClusterID))
	utilruntime.Must(err)

	fc, err := foreignclusterutils.GetForeignClusterByID(ctx, h.cl, h.remoteClusterIdentity.ClusterID)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			s.Fail(fmt.Sprintf("Failed to unpeer cluster %s (%s): %v",
				h.remoteClusterIdentity.ClusterName, h.remoteClusterIdentity.ClusterID, err))
			return err
		}
		s.Success(fmt.Sprintf("Cluster unpeer triggered for %s (%s)",
			h.remoteClusterIdentity.ClusterName, h.remoteClusterIdentity.ClusterID))
		return nil
	}

	fc.Spec.OutgoingPeeringEnabled = discoveryv1alpha1.PeeringEnabledNo
	if err = h.cl.Update(ctx, fc); err != nil {
		s.Fail(fmt.Sprintf("Failed to unpeer cluster %s (%s): %v",
			h.remoteClusterIdentity.ClusterName, h.remoteClusterIdentity.ClusterID, err))
		return err
	}

	s.Success(fmt.Sprintf("Cluster unpeer triggered for %s (%s)",
		h.remoteClusterIdentity.ClusterName, h.remoteClusterIdentity.ClusterID))
	return nil
}

func getClient(file string) (client.Client, error) {
	conf, err := utils.GetRestConfig(file)
	if err != nil {
		return nil, err
	}

	return client.New(conf, client.Options{})
}

func getKClient(file string) (kubernetes.Interface, error) {
	conf, err := utils.GetRestConfig(file)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(conf)
}
