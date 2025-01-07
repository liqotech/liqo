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

package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	certificates "k8s.io/api/certificates/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	mapsctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/namespacemap-controller"
	nsoffctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/namespaceoffloading-controller"
	nodefailurectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/nodefailure-controller"
	podstatusctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/podstatus-controller"
	shadowepsctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/shadowendpointslice-controller"
	shadowpodctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/shadowpod-controller"
	liqostorageprovisioner "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/storageprovisioner"
	virtualnodectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/virtualnode-controller"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/csr"
)

// OffloadingOption defines the options to setup the offloading module.
type OffloadingOption struct {
	Clientset                   *kubernetes.Clientset
	LocalClusterID              liqov1beta1.ClusterID
	NamespaceManager            tenantnamespace.Manager
	EnableStorage               bool
	VirtualStorageClassName     string
	RealStorageClassName        string
	StorageNamespace            string
	EnableNodeFailureController bool
	ShadowPodWorkers            int
	ShadowEndpointSliceWorkers  int
	ResyncPeriod                time.Duration
}

// SetupOffloadingModule setup the offloading module and initializes its controllers.
func SetupOffloadingModule(ctx context.Context, mgr manager.Manager, opts *OffloadingOption) error {
	virtualNodeReconciler, err := virtualnodectrl.NewVirtualNodeReconciler(
		ctx,
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("virtualnode-controller"),
		opts.LocalClusterID,
		opts.NamespaceManager,
	)
	if err != nil {
		klog.Errorf("Unable to create the virtualnode reconciler: %v", err)
		return err
	}
	if err = virtualNodeReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to setup the virtualnode reconciler: %v", err)
		return err
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
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	if err = shadowPodReconciler.SetupWithManager(mgr, opts.ShadowPodWorkers); err != nil {
		klog.Errorf("Unable to setup the shadowpod reconciler: %v", err)
		return err
	}

	shadowEpsReconciler := &shadowepsctrl.Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	if err = shadowEpsReconciler.SetupWithManager(ctx, mgr, opts.ShadowEndpointSliceWorkers); err != nil {
		klog.Errorf("Unable to setup the shadowendpointslice reconciler: %v", err)
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
