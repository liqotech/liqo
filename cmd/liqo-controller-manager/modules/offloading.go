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

package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	certificates "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	liqocontrollermanager "github.com/liqotech/liqo/pkg/liqo-controller-manager"
	gatewayapistatusctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/gatewayapistatus-controller"
	mapsctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/namespacemap-controller"
	nsoffctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/namespaceoffloading-controller"
	nodefailurectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/nodefailure-controller"
	podstatusctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/podstatus-controller"
	shadowepsctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/shadowendpointslice-controller"
	shadowingressctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/shadowingressstatus-controller"
	shadowpodctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/shadowpod-controller"
	liqostorageprovisioner "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/storageprovisioner"
	virtualnodectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/virtualnode-controller"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/csr"
	gwutils "github.com/liqotech/liqo/pkg/utils/gatewayapi"
)

// OffloadingOption defines the options to setup the offloading module.
type OffloadingOption struct {
	Clientset                   *kubernetes.Clientset
	LocalClusterID              liqov1beta1.ClusterID
	NamespaceManager            tenantnamespace.Manager
	LiqoNamespace               string
	LocalPodCIDRs               []string
	VkOptsDefaultTemplate       *corev1.ObjectReference
	EnableStorage               bool
	VirtualStorageClassName     string
	RealStorageClassName        string
	StorageNamespace            string
	EnableNodeFailureController bool
	ShadowPodWorkers            int
	ShadowEndpointSliceWorkers  int
	DenyDirectConnections       bool
	ShadowIngressStatusWorkers  int
	GatewayAPIStatusWorkers     int
	ResyncPeriod                time.Duration
}

// NewOffloadingOption creates a new OffloadingOption with the given parameters.
func NewOffloadingOption(clientset *kubernetes.Clientset, localClusterID liqov1beta1.ClusterID,
	namespaceManager tenantnamespace.Manager, opts *liqocontrollermanager.Options) (*OffloadingOption, error) {
	var vkOptsDefaultTemplate *corev1.ObjectReference
	if opts.VkOptionsDefaultTemplate != "" {
		var err error
		vkOptsDefaultTemplate, err = argsutils.GetObjectRefFromNamespacedName(opts.VkOptionsDefaultTemplate)
		if err != nil {
			return nil, fmt.Errorf("parsing the namespaced name of the virtual-kubelet options template %q: %w",
				opts.VkOptionsDefaultTemplate, err)
		}
	}

	return &OffloadingOption{
		Clientset:                   clientset,
		LocalClusterID:              localClusterID,
		NamespaceManager:            namespaceManager,
		LiqoNamespace:               opts.LiqoNamespace,
		LocalPodCIDRs:               opts.LocalPodCIDRs,
		VkOptsDefaultTemplate:       vkOptsDefaultTemplate,
		EnableStorage:               opts.EnableStorage,
		VirtualStorageClassName:     opts.VirtualStorageClassName,
		RealStorageClassName:        opts.RealStorageClassName,
		StorageNamespace:            opts.StorageNamespace,
		EnableNodeFailureController: opts.EnableNodeFailureController,
		ShadowPodWorkers:            opts.ShadowPodWorkers,
		ShadowEndpointSliceWorkers:  opts.ShadowEndpointSliceWorkers,
		DenyDirectConnections:       opts.DenyDirectConnections,
		ShadowIngressStatusWorkers:  opts.ShadowIngressStatusWorkers,
		GatewayAPIStatusWorkers:     opts.GatewayAPIStatusWorkers,
		ResyncPeriod:                opts.ResyncPeriod,
	}, nil
}

// SetupOffloadingModule setup the offloading module and initializes its controllers.
func SetupOffloadingModule(ctx context.Context, mgr manager.Manager, opts *OffloadingOption) error {
	virtualNodeReconciler, err := virtualnodectrl.NewVirtualNodeReconciler(
		ctx,
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("virtualnode-controller"),
		opts.NamespaceManager,
		virtualnodectrl.VirtualNodeReconcilerOptions{
			HomeClusterID:         opts.LocalClusterID,
			LiqoNamespace:         opts.LiqoNamespace,
			LocalPodCIDRs:         opts.LocalPodCIDRs,
			VkOptsDefaultTemplate: opts.VkOptsDefaultTemplate,
		},
	)
	if err != nil {
		return fmt.Errorf("creating the virtualnode reconciler: %w", err)
	}
	if err = virtualNodeReconciler.SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("setting up the virtualnode reconciler: %w", err)
	}

	namespaceMapReconciler := &mapsctrl.NamespaceMapReconciler{
		Client: mgr.GetClient(),
	}
	if err = namespaceMapReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to setup the namespacemap reconciler: %v", err)
		return err
	}

	namespaceOffloadingReconciler := &nsoffctrl.NamespaceOffloadingReconciler{
		Client:       mgr.GetClient(),
		Recorder:     mgr.GetEventRecorderFor("namespaceoffloading-controller"),
		LocalCluster: opts.LocalClusterID,
	}
	if err = namespaceOffloadingReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to setup the namespaceoffloading reconciler: %v", err)
		return err
	}

	shadowPodReconciler := &shadowpodctrl.Reconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("shadowpod-controller"),
	}
	if err = shadowPodReconciler.SetupWithManager(mgr, opts.ShadowPodWorkers); err != nil {
		klog.Errorf("Unable to setup the shadowpod reconciler: %v", err)
		return err
	}

	shadowEpsReconciler := &shadowepsctrl.Reconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		DenyDirectConnections: opts.DenyDirectConnections,
	}
	if err = shadowEpsReconciler.SetupWithManager(ctx, mgr, opts.ShadowEndpointSliceWorkers); err != nil {
		klog.Errorf("Unable to setup the shadowendpointslice reconciler: %v", err)
		return err
	}

	shadowIngressStatusReconciler := &shadowingressctrl.Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	if err = shadowIngressStatusReconciler.SetupWithManager(mgr, opts.ShadowIngressStatusWorkers); err != nil {
		klog.Errorf("Unable to setup the shadowingressstatus reconciler: %v", err)
		return err
	}

	if err = setupGatewayAPIStatusControllers(ctx, mgr, opts); err != nil {
		return err
	}

	if opts.EnableStorage {
		liqoProvisioner, err := liqostorageprovisioner.NewLiqoLocalStorageProvisioner(ctx, mgr.GetClient(),
			opts.VirtualStorageClassName, opts.StorageNamespace, opts.RealStorageClassName)
		if err != nil {
			klog.Errorf("unable to start the liqo storage provisioner: %v", err)
			return err
		}
		provisionController := controller.NewProvisionController(opts.Clientset, consts.StorageProvisionerName, liqoProvisioner,
			controller.LeaderElection(false),
		)
		if err = mgr.Add(liqostorageprovisioner.StorageControllerRunnable{Ctrl: provisionController}); err != nil {
			klog.Errorf("unable to add the storage provisioner controller to the manager: %v", err)
			return err
		}
	}

	// Start the handler to approve the virtual kubelet certificate signing requests.
	csrWatcher := csr.NewWatcher(opts.Clientset, opts.ResyncPeriod, labels.Everything(), fields.Everything())
	csrWatcher.RegisterHandler(csr.ApproverHandler(opts.Clientset, "LiqoApproval", "This CSR was approved by Liqo",
		// Approve only the CSRs for a requestor living in a liqo tenant namespace (based on the prefix).
		// This is far from elegant, but the client-go utility generating the CSRs does not allow to customize the labels.
		func(csr *certificates.CertificateSigningRequest) bool {
			return strings.HasPrefix(csr.Spec.Username, fmt.Sprintf("system:serviceaccount:%v-", tenantnamespace.NamePrefix))
		}))
	csrWatcher.Start(ctx)

	podStatusReconciler := &podstatusctrl.PodStatusReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	if err = podStatusReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to setup the podstatus reconciler: %v", err)
		return err
	}

	if opts.EnableNodeFailureController {
		nodeFailureReconciler := &nodefailurectrl.NodeFailureReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}
		if err = nodeFailureReconciler.SetupWithManager(mgr); err != nil {
			klog.Errorf("Unable to setup the nodefailure reconciler: %v", err)
			return err
		}
	}

	return nil
}

// setupGatewayAPIStatusControllers sets up the controllers aggregating the status of the Gateway API resources reflected
// to the remote clusters. Each controller is set up only if the corresponding resources are available in the local cluster,
// as the controller would otherwise fail to start, preventing the whole manager from starting.
func setupGatewayAPIStatusControllers(ctx context.Context, mgr manager.Manager, opts *OffloadingOption) error {
	available, err := gwutils.Detect(opts.Clientset.Discovery())
	if err != nil {
		klog.Errorf("Unable to detect the Gateway API resources: %v", err)
		return err
	}

	if available.Has(gwutils.GatewayClassesGVR) {
		gatewayClassReconciler := &gatewayapistatusctrl.GatewayClassReconciler{Client: mgr.GetClient()}
		if err := gatewayClassReconciler.SetupWithManager(mgr); err != nil {
			klog.Errorf("Unable to setup the gatewayclass status reconciler: %v", err)
			return err
		}
	}

	if available.Has(gwutils.GatewayClassesGVR) && available.Has(gwutils.GatewaysGVR) {
		gatewayReconciler := &gatewayapistatusctrl.GatewayReconciler{Client: mgr.GetClient()}
		if err := gatewayReconciler.SetupWithManager(ctx, mgr, opts.GatewayAPIStatusWorkers); err != nil {
			klog.Errorf("Unable to setup the shadowgatewaystatus reconciler: %v", err)
			return err
		}
	}

	routes := map[offloadingv1beta1.RouteKind]schema.GroupVersionResource{
		offloadingv1beta1.HTTPRouteKind: gwutils.HTTPRoutesGVR,
		offloadingv1beta1.GRPCRouteKind: gwutils.GRPCRoutesGVR,
	}
	indexed := false
	for kind, gvr := range routes {
		if !available.Has(gvr) {
			klog.Infof("Gateway API resource %s not available: status aggregation disabled", gvr.GroupResource())
			continue
		}

		if !indexed {
			if err := gatewayapistatusctrl.SetupRouteIndexer(ctx, mgr); err != nil {
				klog.Errorf("Unable to setup the shadowroutestatus indexer: %v", err)
				return err
			}
			indexed = true
		}

		routeReconciler := &gatewayapistatusctrl.RouteReconciler{Client: mgr.GetClient(), Kind: kind}
		if err := routeReconciler.SetupWithManager(mgr, opts.GatewayAPIStatusWorkers); err != nil {
			klog.Errorf("Unable to setup the %s status reconciler: %v", kind, err)
			return err
		}
	}

	return nil
}
