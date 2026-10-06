// Copyright 2019-2026 The Liqo Authors
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

package provider

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	gwclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	gwutils "github.com/liqotech/liqo/pkg/utils/gatewayapi"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/gatewayapi"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/resources"
)

// gatewayAPIResource associates a Gateway API resource with the corresponding reflector constructor.
type gatewayAPIResource struct {
	resource  resources.ResourceReflected
	gvr       schema.GroupVersionResource
	reflector func(*offloadingv1beta1.ReflectorConfig, *gatewayapi.Config) manager.Reflector
}

var gatewayAPIResources = []gatewayAPIResource{
	{resource: resources.Gateway, gvr: gwutils.GatewaysGVR, reflector: gatewayapi.NewGatewayReflector},
	{resource: resources.HTTPRoute, gvr: gwutils.HTTPRoutesGVR, reflector: gatewayapi.NewHTTPRouteReflector},
	{resource: resources.GRPCRoute, gvr: gwutils.GRPCRoutesGVR, reflector: gatewayapi.NewGRPCRouteReflector},
	{resource: resources.ReferenceGrant, gvr: gwutils.ReferenceGrantsGVR, reflector: gatewayapi.NewReferenceGrantReflector},
}

// setupGatewayAPIReflection registers the Gateway API reflectors, depending on the resources available in both clusters.
// Informers are never started for resources missing in a cluster, since they would never sync, blocking the reflection
// of the whole namespace. Resources available in the local cluster only are handled in degraded mode, generating warning events.
func setupGatewayAPIReflection(cfg *InitConfig, localClient, remoteClient kubernetes.Interface, reflectionManager manager.Manager) error {
	local, err := gwutils.Detect(localClient.Discovery())
	if err != nil {
		return fmt.Errorf("failed to detect the Gateway API resources in the local cluster: %w", err)
	}
	remote, err := gwutils.Detect(remoteClient.Discovery())
	if err != nil {
		return fmt.Errorf("failed to detect the Gateway API resources in remote cluster %q: %w", forge.RemoteCluster, err)
	}

	// The routes reflected to the remote cluster report the addresses of the shared Gateway they are attached to.
	var reflectedRoutes []schema.GroupResource
	for _, gvr := range []schema.GroupVersionResource{gwutils.HTTPRoutesGVR, gwutils.GRPCRoutesGVR} {
		if gatewayapi.SupportFor(gvr, local, remote) == gatewayapi.SupportFull {
			reflectedRoutes = append(reflectedRoutes, gvr.GroupResource())
		}
	}

	// The clients are configured only if at least one resource is available, and left as untyped nil otherwise.
	var localGateway, remoteGateway gwclient.Interface
	for _, res := range gatewayAPIResources {
		support := gatewayapi.SupportFor(res.gvr, local, remote)
		switch support {
		case gatewayapi.SupportNone:
			klog.Infof("Gateway API resource %s not available in the local cluster: reflection disabled", res.gvr.GroupResource())
			continue
		case gatewayapi.SupportDegraded:
			klog.Warningf("Gateway API resource %s not available in remote cluster %q (virtual node %q): local objects will not be reflected",
				res.gvr.GroupResource(), forge.RemoteCluster, forge.LiqoNodeName)
		case gatewayapi.SupportFull:
			if remoteGateway == nil {
				remoteGateway = gwclient.NewForConfigOrDie(cfg.RemoteConfig)
			}
		}

		if localGateway == nil {
			localGateway = gwclient.NewForConfigOrDie(cfg.LocalConfig)
		}
		reflectionManager.With(res.reflector(ptr.To(cfg.ReflectorsConfigs[res.resource]), &gatewayapi.Config{
			Support:              support,
			GatewaysAvailable:    local.Has(gwutils.GatewaysGVR),
			SharedGatewayEnabled: cfg.RemoteSharedGatewayEnabled,
			VirtualGatewayClass:  cfg.VirtualGatewayClassName,
			RemoteGatewayClass:   cfg.RemoteRealGatewayClassName,
			ReflectedRoutes:      reflectedRoutes,
		}))
	}

	reflectionManager.WithGatewayAPI(localGateway, remoteGateway)

	// The routes of the kinds not reflected by Liqo would be otherwise silently ignored: a warning event is generated for each of them.
	// Failures are not fatal, since the reflection of the other resources is not affected.
	routes, err := gwutils.DetectUnsupportedRoutes(localClient.Discovery())
	if err != nil {
		klog.Warningf("Failed to detect the Gateway API routes not supported by the reflection: %v", err)
		return nil
	}
	if len(routes) > 0 {
		for _, reflector := range gatewayapi.NewUnsupportedRouteReflectors(localClient, metadata.NewForConfigOrDie(cfg.LocalConfig),
			routes, ptr.To(cfg.ReflectorsConfigs[resources.HTTPRoute])) {
			reflectionManager.With(reflector)
		}
	}
	return nil
}
