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

package storage

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	storagev1listers "k8s.io/client-go/listers/storage/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	liqostorageprovisioner "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/storageprovisioner"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/virtualkubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	// PersistentVolumeClaimReflectorName -> The name associated with the PersistentVolumeClaim reflector.
	PersistentVolumeClaimReflectorName = "PersistentVolumeClaim"
)

var _ manager.NamespacedReflector = (*NamespacedPersistentVolumeClaimReflector)(nil)

// NamespacedPersistentVolumeClaimReflector manages the PersistentVolumeClaim reflection for a given pair of local and remote namespaces.
type NamespacedPersistentVolumeClaimReflector struct {
	generic.NamespacedReflector

	classes                             storagev1listers.StorageClassLister
	volumes                             corev1listers.PersistentVolumeLister
	nodes                               corev1listers.NodeLister
	localPersistentVolumeClaims         corev1listers.PersistentVolumeClaimNamespaceLister
	remotePersistentVolumeClaims        corev1listers.PersistentVolumeClaimNamespaceLister
	remotePersistentVolumesClaimsClient corev1clients.PersistentVolumeClaimInterface
	localPersistentVolumesClient        corev1clients.PersistentVolumeInterface
	localPersistentVolumeClaimsClient   corev1clients.PersistentVolumeClaimInterface

	provisionerName string
	storageEnabled  bool

	virtualStorageClassName    string
	remoteRealStorageClassName string
}

// NewPersistentVolumeClaimReflector returns a new PersistentVolumeClaimReflector instance.
func NewPersistentVolumeClaimReflector(virtualStorageClassName, remoteRealStorageClassName string,
	storageEnabled bool, reflectorConfig *offloadingv1beta1.ReflectorConfig) manager.Reflector {
	return generic.NewReflector(PersistentVolumeClaimReflectorName,
		NewNamespacedPersistentVolumeClaimReflector(virtualStorageClassName, remoteRealStorageClassName, storageEnabled),
		generic.WithoutFallback(), reflectorConfig.NumWorkers, offloadingv1beta1.CustomLiqo, generic.ConcurrencyModeLeader)
}

// NewNamespacedPersistentVolumeClaimReflector returns a function generating NamespacedPersistentVolumeClaimReflector instances.
func NewNamespacedPersistentVolumeClaimReflector(virtualStorageClassName,
	remoteRealStorageClassName string, storageEnabled bool) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return func(opts *options.NamespacedOpts) manager.NamespacedReflector {
		local := opts.LocalFactory.Core().V1().PersistentVolumeClaims()
		remote := opts.RemoteFactory.Core().V1().PersistentVolumeClaims()
		localStorage := opts.LocalFactory.Storage().V1().StorageClasses()
		localVolumes := opts.LocalFactory.Core().V1().PersistentVolumes()
		localNodes := opts.LocalFactory.Core().V1().Nodes()

		local.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		remote.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))

		return &NamespacedPersistentVolumeClaimReflector{
			NamespacedReflector: generic.NewNamespacedReflector(opts, PersistentVolumeClaimReflectorName),

			classes:                             localStorage.Lister(),
			volumes:                             localVolumes.Lister(),
			nodes:                               localNodes.Lister(),
			localPersistentVolumeClaims:         local.Lister().PersistentVolumeClaims(opts.LocalNamespace),
			remotePersistentVolumeClaims:        remote.Lister().PersistentVolumeClaims(opts.RemoteNamespace),
			remotePersistentVolumesClaimsClient: opts.RemoteClient.CoreV1().PersistentVolumeClaims(opts.RemoteNamespace),
			localPersistentVolumesClient:        opts.LocalClient.CoreV1().PersistentVolumes(),
			localPersistentVolumeClaimsClient:   opts.LocalClient.CoreV1().PersistentVolumeClaims(opts.LocalNamespace),

			provisionerName: consts.StorageProvisionerName,
			storageEnabled:  storageEnabled,

			virtualStorageClassName:    virtualStorageClassName,
			remoteRealStorageClassName: remoteRealStorageClassName,
		}
	}
}

// Handle reconciles PersistentVolumeClaim objects.
func (npvcr *NamespacedPersistentVolumeClaimReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	// Retrieve the local and remote objects (only not found errors can occur).
	klog.V(4).Infof("Handling reflection of local PersistentVolumeClaim %q (remote: %q)", npvcr.LocalRef(name), npvcr.RemoteRef(name))
	local, lerr := npvcr.localPersistentVolumeClaims.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	remote, rerr := npvcr.remotePersistentVolumeClaims.Get(name)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	tracer.Step("Retrieved the local and remote objects")

	// Abort the reflection if the remote object is not managed by us, as we do not want to mutate others' objects.
	if rerr == nil && !forge.IsReflected(remote) {
		if lerr == nil { // Do not output the warning event in case the event was triggered by the remote object (i.e., the local one does not exists).
			klog.Infof("Skipping reflection of local PersistentVolumeClaim %q as remote already exists and is not managed by us", npvcr.LocalRef(name))
			npvcr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionAlreadyExistsMsg())
		}
		return nil
	}
	tracer.Step("Performed the sanity checks")

	// The local PersistentVolumeClaim does no longer exist. Ensure it is also absent from the remote cluster.
	if kerrors.IsNotFound(lerr) {
		defer tracer.Step("Ensured the absence of the remote object")
		if !kerrors.IsNotFound(rerr) {
			klog.V(4).Infof("Deleting remote PersistentVolumeClaim %q, since local %q does no longer exist", npvcr.RemoteRef(name), npvcr.LocalRef(name))
			return npvcr.DeleteRemote(ctx, npvcr.remotePersistentVolumesClaimsClient, PersistentVolumeClaimReflectorName, name, remote.GetUID())
		}

		klog.V(4).Infof("Local PersistentVolumeClaim %q and remote PersistentVolumeClaim %q both vanished", npvcr.LocalRef(name), npvcr.RemoteRef(name))
		return nil
	}

	// DeepCopy the local object to allow modifications.
	local = local.DeepCopy()

	// Check if we should provision storage for that PVC. We have to check if no volume is already provisioned and the storage class is the expected one.
	if should, err := npvcr.shouldProvision(local); err != nil {
		klog.V(4).Infof("Error checking if should provision a local PersistentVolumeClaim %q: %v", npvcr.LocalRef(name), err.Error())
		npvcr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return err
	} else if !should {
		klog.V(4).Infof("Skipping PersistentVolumeClaim %q since we should not provision it", npvcr.LocalRef(name))
		return nil
	}

	// Check if the storage is enabled on the current node.
	if !npvcr.storageEnabled {
		msg := fmt.Sprintf("Required PersistentVolumeClaim %q rescheduling since storage is not enabled on the current node", npvcr.LocalRef(name))
		klog.V(4).Info(msg)
		npvcr.Event(local, corev1.EventTypeWarning, "ReschedulingRequired", msg)
		// The provisioner may remove
		// annSelectedNode to notify scheduler to reschedule again.
		delete(local.Annotations, annSelectedNode)
		_, err := npvcr.localPersistentVolumeClaimsClient.Update(ctx, local, metav1.UpdateOptions{})
		return err
	}
	tracer.Step("Ensured to have to provision the volume")

	// Run the actual remote PVC reconciliation.
	// Similarly to what a storage class controller does, we provide a local PV for a local PVC.
	// https://github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/blob/v7.0.1/controller/controller.go#L1275
	// In addition, we use the provisionFunc to ensure the provisioning of the remote PVC.
	state, err := npvcr.provisionClaimOperation(ctx, local,
		func(_ context.Context, options controller.ProvisionOptions) (*corev1.PersistentVolume, controller.ProvisioningState, error) {
			if clusterID, found := utils.GetNodeClusterID(options.SelectedNode); !found || clusterID != string(forge.RemoteCluster) {
				return nil, controller.ProvisioningFinished, &controller.IgnoredError{Reason: "this provisioner is not provisioning storage on that node"}
			}

			pv, state, err := liqostorageprovisioner.ProvisionRemotePVC(ctx,
				options, npvcr.RemoteNamespace(), npvcr.remoteRealStorageClassName,
				npvcr.remotePersistentVolumeClaims, npvcr.remotePersistentVolumesClaimsClient,
				npvcr.ForgingOpts)
			if err == nil && state == controller.ProvisioningFinished {
				local.Spec.VolumeName = options.PVName
			}
			return pv, state, err
		})
	if err != nil {
		klog.Errorf("Error provisioning the remote PersistentVolumeClaim %q: %v", npvcr.RemoteRef(name), err.Error())
		npvcr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return err
	}
	npvcr.Event(local, corev1.EventTypeNormal, forge.EventSuccessfulReflection, forge.EventSuccessfulReflectionMsg())
	tracer.Step("Remote mutation created")

	klog.V(4).Infof("Handle of local PersistentVolumeClaim %q (remote: %q) finished with state %q", npvcr.LocalRef(name), npvcr.RemoteRef(name), state)
	switch state {
	case controller.ProvisioningFinished, controller.ProvisioningNoChange, controller.ProvisioningReschedule:
		if _, err = npvcr.localPersistentVolumeClaimsClient.Update(ctx, local, metav1.UpdateOptions{}); err != nil {
			if !kerrors.IsConflict(err) {
				npvcr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedStatusReflectionMsg(err))
			}
			return err
		}
		npvcr.Event(local, corev1.EventTypeNormal, forge.EventSuccessfulReflection, forge.EventSuccessfulStatusReflectionMsg())
		return nil
	case controller.ProvisioningInBackground:
		return fmt.Errorf("provisioning of local PersistentVolumeClaim %q is still in progress", npvcr.LocalRef(name))
	default:
		return fmt.Errorf("unknown state %v", state)
	}
}

// List lists all PersistentVolumeClaims in the local cluster.
func (npvcr *NamespacedPersistentVolumeClaimReflector) List() ([]interface{}, error) {
	return virtualkubelet.List[virtualkubelet.Lister[*corev1.PersistentVolumeClaim], *corev1.PersistentVolumeClaim](
		npvcr.localPersistentVolumeClaims,
		npvcr.remotePersistentVolumeClaims,
	)
}
