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

package route

import (
	"context"
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/tunnel/common"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// GenerateRouteConfigurationName generates the name of the RouteConfiguration object.
func GenerateRouteConfigurationName(cfg *networkingv1alpha1.Configuration) string {
	return fmt.Sprintf("%s-gw-ext", cfg.Name)
}

// GetRemoteClusterID returns the remote cluster ID of the Configuration.
func GetRemoteClusterID(cfg *networkingv1alpha1.Configuration) (string, error) {
	if cfg.GetLabels() == nil {
		return "", fmt.Errorf("configuration %s/%s has no labels", cfg.Namespace, cfg.Name)
	}
	remoteID, ok := cfg.GetLabels()[consts.RemoteClusterID]
	if !ok {
		return "", fmt.Errorf("configuration %s/%s has no remote cluster ID label", cfg.Namespace, cfg.Name)
	}
	return remoteID, nil
}

// enforceRouteConfigurationPresence creates or updates a RouteConfiguration object.
func enforeRouteConfigurationPresence(ctx context.Context, cl client.Client, scheme *runtime.Scheme,
	cfg *networkingv1alpha1.Configuration) error {
	remoteClusterID, err := GetRemoteClusterID(cfg)
	if err != nil {
		return err
	}

	mode, err := GetGatewayMode(ctx, cl, remoteClusterID)
	if err != nil {
		return err
	}
	// If the Gateway is not already present, we are not able to understand if it will be a server or a client
	if mode == "" {
		return nil
	}

	remoteInterfaceIP, err := common.GetRemoteInterfaceIP(mode)
	if err != nil {
		return err
	}

	routecfg := &networkingv1alpha1.RouteConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GenerateRouteConfigurationName(cfg),
			Namespace: cfg.Namespace,
		},
	}

	internalNodes, err := getters.ListInternalNodesByLabels(ctx, cl, labels.Everything())
	if err != nil {
		return err
	}

	_, err = controllerutil.CreateOrUpdate(ctx, cl, routecfg,
		forgeMutateRouteConfiguration(cfg, routecfg, scheme, remoteClusterID, remoteInterfaceIP, internalNodes))
	return err
}

// forgeMutateRouteConfiguration mutates a RouteConfiguration object.
func forgeMutateRouteConfiguration(cfg *networkingv1alpha1.Configuration,
	routecfg *networkingv1alpha1.RouteConfiguration, scheme *runtime.Scheme,
	remoteClusterID string, remoteInterfaceIP string, internalNodes *networkingv1alpha1.InternalNodeList) func() error {
	return func() error {
		var err error

		if err = controllerutil.SetOwnerReference(cfg, routecfg, scheme); err != nil {
			return err
		}

		routecfg.ObjectMeta.Labels = gateway.ForgeRouteExternalTargetLabels(remoteClusterID)
		if err != nil {
			return err
		}

		routecfg.Spec = networkingv1alpha1.RouteConfigurationSpec{
			Table: networkingv1alpha1.Table{
				Name: cfg.Name,
			},
		}

		for i := range internalNodes.Items {
			routecfg.Spec.Table.Rules = append(routecfg.Spec.Table.Rules,
				[]networkingv1alpha1.Rule{
					{
						Iif: &internalNodes.Items[i].Spec.Interface.Gateway.Name,
						Dst: &cfg.Spec.Remote.CIDR.Pod,
						Routes: []networkingv1alpha1.Route{
							{
								Dst: &cfg.Spec.Remote.CIDR.Pod,
								Gw:  ptr.To(networkingv1alpha1.IP(remoteInterfaceIP)),
							},
						},
					},
					{
						Iif: &internalNodes.Items[i].Spec.Interface.Gateway.Name,
						Dst: &cfg.Spec.Remote.CIDR.External,
						Routes: []networkingv1alpha1.Route{
							{
								Dst: &cfg.Spec.Remote.CIDR.External,
								Gw:  ptr.To(networkingv1alpha1.IP(remoteInterfaceIP)),
							},
						},
					},
				}...)
		}
		return nil
	}
}

// GetGatewayMode returns the mode of the Gateway related to the Configuration.
func GetGatewayMode(ctx context.Context, cl client.Client, remoteClusterID string) (gateway.Mode, error) {
	gwclient, err := getters.GetGatewayClientByClusterID(ctx, cl, &v1alpha1.ClusterIdentity{ClusterID: remoteClusterID})
	if err != nil && !kerrors.IsNotFound(err) {
		return "", err
	}

	gwserver, err := getters.GetGatewayServerByClusterID(ctx, cl, &v1alpha1.ClusterIdentity{ClusterID: remoteClusterID})
	if err != nil && !kerrors.IsNotFound(err) {
		return "", err
	}

	switch {
	case gwclient == nil && gwserver == nil:
		return "", nil
	case gwclient != nil && gwserver != nil:
		return "", fmt.Errorf("multiple Gateways found for cluster %s", remoteClusterID)
	case gwclient == nil && gwserver != nil:
		return gateway.ModeServer, nil
	case gwclient != nil && gwserver == nil:
		return gateway.ModeClient, nil
	}

	return "", fmt.Errorf("unable to determine Gateway mode for cluster %s", remoteClusterID)
}
