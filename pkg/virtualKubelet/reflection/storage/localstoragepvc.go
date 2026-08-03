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

package storage

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/util"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/maps"
	"github.com/liqotech/liqo/pkg/utils/virtualkubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	// LocalStoragePVCReflectorName -> The name associated with the local-storage PersistentVolumeClaim reflector.
	LocalStoragePVCReflectorName = "LocalStoragePVC"
	// LocalStorageClassName -> The name of the StorageClass used for local volumes.
	LocalStorageClassName = "local-storage"
)

var _ manager.NamespacedReflector = (*NamespacedLocalStoragePVCReflector)(nil)

// NamespacedLocalStoragePVCReflector manages the reflection of local-storage PersistentVolumeClaims
// (and their bound PersistentVolumes) for a given pair of local and remote namespaces.
type NamespacedLocalStoragePVCReflector struct {
	generic.NamespacedReflector

	localPersistentVolumeClaims         corev1listers.PersistentVolumeClaimNamespaceLister
	remotePersistentVolumeClaims        corev1listers.PersistentVolumeClaimNamespaceLister
	localPersistentVolumes              corev1listers.PersistentVolumeLister
	remotePersistentVolumes             corev1listers.PersistentVolumeLister
	remotePersistentVolumesClaimsClient corev1clients.PersistentVolumeClaimInterface
	remotePersistentVolumesClient       corev1clients.PersistentVolumeInterface
}

// NewLocalStoragePVCReflector returns a new LocalStoragePVCReflector instance.
func NewLocalStoragePVCReflector(reflectorConfig *offloadingv1beta1.ReflectorConfig) manager.Reflector {
	return generic.NewReflector(LocalStoragePVCReflectorName,
		NewNamespacedLocalStoragePVCReflector(),
		generic.WithoutFallback(), reflectorConfig.NumWorkers, offloadingv1beta1.CustomLiqo, generic.ConcurrencyModeLeader)
}

// NewNamespacedLocalStoragePVCReflector returns a function generating NamespacedLocalStoragePVCReflector instances.
func NewNamespacedLocalStoragePVCReflector() func(*options.NamespacedOpts) manager.NamespacedReflector {
	return func(opts *options.NamespacedOpts) manager.NamespacedReflector {
		local := opts.LocalFactory.Core().V1().PersistentVolumeClaims()
		remote := opts.RemoteFactory.Core().V1().PersistentVolumeClaims()
		localVolumes := opts.LocalFactory.Core().V1().PersistentVolumes()
		remoteVolumes := opts.RemoteFactory.Core().V1().PersistentVolumes()

		_, err := local.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)
		_, err = remote.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)
		_, err = localVolumes.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)
		_, err = remoteVolumes.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)

		return &NamespacedLocalStoragePVCReflector{
			NamespacedReflector: generic.NewNamespacedReflector(opts, LocalStoragePVCReflectorName),

			localPersistentVolumeClaims:         local.Lister().PersistentVolumeClaims(opts.LocalNamespace),
			remotePersistentVolumeClaims:        remote.Lister().PersistentVolumeClaims(opts.RemoteNamespace),
			localPersistentVolumes:              localVolumes.Lister(),
			remotePersistentVolumes:             remoteVolumes.Lister(),
			remotePersistentVolumesClaimsClient: opts.RemoteClient.CoreV1().PersistentVolumeClaims(opts.RemoteNamespace),
			remotePersistentVolumesClient:       opts.RemoteClient.CoreV1().PersistentVolumes(),
		}
	}
}

// Handle reconciles local-storage PersistentVolumeClaim objects.
func (nls *NamespacedLocalStoragePVCReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	klog.V(4).Infof("Handling reflection of local PersistentVolumeClaim %q (remote: %q)", nls.LocalRef(name), nls.RemoteRef(name))
	local, lerr := nls.localPersistentVolumeClaims.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	remote, rerr := nls.remotePersistentVolumeClaims.Get(name)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	tracer.Step("Retrieved the local and remote objects")

	// Abort the reflection if the remote object is not managed by us.
	if rerr == nil && !forge.IsReflected(remote) {
		if lerr == nil {
			klog.Infof("Skipping reflection of local PersistentVolumeClaim %q as remote already exists and is not managed by us", nls.LocalRef(name))
			nls.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionAlreadyExistsMsg())
		}
		return nil
	}
	tracer.Step("Performed the sanity checks")

	// The local PersistentVolumeClaim no longer exists. Ensure it is also absent from the remote cluster.
	if kerrors.IsNotFound(lerr) {
		defer tracer.Step("Ensured the absence of the remote object")
		if !kerrors.IsNotFound(rerr) {
			klog.V(4).Infof("Deleting remote PersistentVolumeClaim %q, since local %q no longer exists", nls.RemoteRef(name), nls.LocalRef(name))
			// Delete the PVC first to release the PV protection finalizer on the bound volume.
			remotePVName := remote.Spec.VolumeName
			if err := nls.DeleteRemote(ctx, nls.remotePersistentVolumesClaimsClient, LocalStoragePVCReflectorName, name, remote.GetUID()); err != nil {
				return err
			}
			// Also clean up the reflected PV if present.
			if remotePVName != "" {
				if err := nls.deleteRemotePV(ctx, remotePVName); err != nil {
					return err
				}
			}
			return nil
		}

		klog.V(4).Infof("Local PersistentVolumeClaim %q and remote PersistentVolumeClaim %q both vanished", nls.LocalRef(name), nls.RemoteRef(name))
		return nil
	}

	// DeepCopy the local object to allow modifications.
	local = local.DeepCopy()

	// Only handle PVCs backed by the local-storage StorageClass.
	if !isLocalStoragePVC(local) {
		klog.V(4).Infof("Skipping PersistentVolumeClaim %q since it is not a local-storage PVC", nls.LocalRef(name))
		return nil
	}
	tracer.Step("Confirmed the PVC uses the local-storage class")

	// Only reflect PVCs that are already bound to a PV.
	if local.Spec.VolumeName == "" {
		klog.V(4).Infof("Skipping reflection of unbound local PersistentVolumeClaim %q", nls.LocalRef(name))
		return nil
	}
	tracer.Step("Confirmed the PVC is bound to a PV")

	// Retrieve the bound PV.
	localPV, err := nls.localPersistentVolumes.Get(local.Spec.VolumeName)
	if err != nil {
		klog.Errorf("Error retrieving bound PersistentVolume %q for local PersistentVolumeClaim %q: %v",
			local.Spec.VolumeName, nls.LocalRef(name), err.Error())
		nls.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return err
	}
	tracer.Step("Retrieved the bound PersistentVolume")

	// Forge and apply the remote PVC.
	remotePVName := remotePVNameFromLocal(localPV)
	remotePVCApply := forgeRemotePVC(local, nls.RemoteNamespace(), remotePVName, nls.ForgingOpts)
	remotePVC, err := nls.remotePersistentVolumesClaimsClient.Apply(ctx, remotePVCApply, forge.ApplyOptions())
	if err != nil {
		klog.Errorf("Error applying remote PersistentVolumeClaim %q: %v", nls.RemoteRef(name), err.Error())
		nls.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return err
	}
	tracer.Step("Applied the remote PersistentVolumeClaim")

	// Forge and apply the remote PV, bound to the remote PVC.
	remotePVApply := forgeRemotePV(localPV, remotePVC, nls.RemoteNamespace(), nls.ForgingOpts)
	if _, err = nls.remotePersistentVolumesClient.Apply(ctx, remotePVApply, forge.ApplyOptions()); err != nil {
		klog.Errorf("Error applying remote PersistentVolume %q: %v", remotePVName, err.Error())
		nls.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return err
	}
	tracer.Step("Applied the remote PersistentVolume")

	nls.Event(local, corev1.EventTypeNormal, forge.EventSuccessfulReflection, forge.EventSuccessfulReflectionMsg())
	klog.V(4).Infof("Handle of local PersistentVolumeClaim %q (remote: %q) finished", nls.LocalRef(name), nls.RemoteRef(name))
	return nil
}

// List lists all local-storage PersistentVolumeClaims in the local cluster.
func (nls *NamespacedLocalStoragePVCReflector) List() ([]interface{}, error) {
	return virtualkubelet.List[virtualkubelet.Lister[*corev1.PersistentVolumeClaim], *corev1.PersistentVolumeClaim](
		nls.localPersistentVolumeClaims,
		nls.remotePersistentVolumeClaims,
	)
}

func (nls *NamespacedLocalStoragePVCReflector) deleteRemotePV(ctx context.Context, name string) error {
	remotePV, err := nls.remotePersistentVolumes.Get(name)
	if kerrors.IsNotFound(err) {
		klog.V(4).Infof("Remote PersistentVolume %q not found, skipping deletion", name)
		return nil
	}
	if err != nil {
		return err
	}

	if !forge.IsReflected(remotePV) {
		klog.V(4).Infof("Skipping deletion of remote PersistentVolume %q since it is not managed by us (labels: %v)", name, remotePV.GetLabels())
		return nil
	}

	klog.V(4).Infof("Deleting remote PersistentVolume %q", name)
	return nls.remotePersistentVolumesClient.Delete(ctx, name, metav1.DeleteOptions{})
}

// isLocalStoragePVC returns whether the given PVC is backed by the local-storage StorageClass.
func isLocalStoragePVC(pvc *corev1.PersistentVolumeClaim) bool {
	className := util.GetPersistentVolumeClaimClass(pvc)
	return className == LocalStorageClassName
}

// remotePVNameFromLocal returns the name to use for the reflected copy of the given local PV.
func remotePVNameFromLocal(localPV *corev1.PersistentVolume) string {
	return fmt.Sprintf("liqo-local-%s", localPV.UID)
}

// forgeRemotePVC forges the apply patch for the reflected PersistentVolumeClaim.
func forgeRemotePVC(local *corev1.PersistentVolumeClaim, remoteNamespace, remotePVName string,
	forgingOpts *forge.ForgingOpts) *corev1apply.PersistentVolumeClaimApplyConfiguration {
	return corev1apply.PersistentVolumeClaim(local.Name, remoteNamespace).
		WithLabels(forge.FilterNotReflected(local.GetLabels(), forgingOpts.LabelsNotReflected)).
		WithLabels(forge.ReflectionLabels()).
		WithAnnotations(forge.FilterNotReflected(filterPVCAnnotations(local.GetAnnotations()), forgingOpts.AnnotationsNotReflected)).
		WithSpec(forgeRemotePVCSpec(local, remotePVName))
}

func forgeRemotePVCSpec(local *corev1.PersistentVolumeClaim, remotePVName string) *corev1apply.PersistentVolumeClaimSpecApplyConfiguration {
	res := corev1apply.PersistentVolumeClaimSpec().
		WithAccessModes(local.Spec.AccessModes...).
		WithVolumeMode(func() corev1.PersistentVolumeMode {
			if local.Spec.VolumeMode != nil {
				return *local.Spec.VolumeMode
			}
			return corev1.PersistentVolumeFilesystem
		}()).
		WithResources(volumeResourceRequirements(local.Spec.Resources)).
		WithVolumeName(remotePVName)

	if local.Spec.StorageClassName != nil && *local.Spec.StorageClassName != "" {
		res.WithStorageClassName(*local.Spec.StorageClassName)
	}

	return res
}

// forgeRemotePV forges the apply patch for the reflected PersistentVolume.
func forgeRemotePV(localPV *corev1.PersistentVolume, remotePVC *corev1.PersistentVolumeClaim,
	remoteNamespace string, forgingOpts *forge.ForgingOpts) *corev1apply.PersistentVolumeApplyConfiguration {
	name := remotePVNameFromLocal(localPV)
	return corev1apply.PersistentVolume(name).
		WithLabels(forge.FilterNotReflected(localPV.GetLabels(), forgingOpts.LabelsNotReflected)).
		WithLabels(forge.ReflectionLabels()).
		WithAnnotations(forge.FilterNotReflected(filterPVAnnotations(localPV.GetAnnotations()), forgingOpts.AnnotationsNotReflected)).
		WithSpec(forgeRemotePVSpec(localPV, remotePVC, remoteNamespace))
}

func forgeRemotePVSpec(localPV *corev1.PersistentVolume, remotePVC *corev1.PersistentVolumeClaim,
	remoteNamespace string) *corev1apply.PersistentVolumeSpecApplyConfiguration {
	res := corev1apply.PersistentVolumeSpec().
		WithCapacity(localPV.Spec.Capacity).
		WithAccessModes(localPV.Spec.AccessModes...).
		WithVolumeMode(func() corev1.PersistentVolumeMode {
			if localPV.Spec.VolumeMode != nil {
				return *localPV.Spec.VolumeMode
			}
			return corev1.PersistentVolumeFilesystem
		}()).
		WithStorageClassName(localPV.Spec.StorageClassName).
		WithClaimRef(corev1apply.ObjectReference().
			WithAPIVersion("v1").
			WithKind("PersistentVolumeClaim").
			WithNamespace(remoteNamespace).
			WithName(remotePVC.Name).
			WithUID(remotePVC.UID)).
		WithPersistentVolumeReclaimPolicy(localPV.Spec.PersistentVolumeReclaimPolicy).
		WithMountOptions(localPV.Spec.MountOptions...).
		WithNodeAffinity(sanitizeNodeAffinity(localPV.Spec.NodeAffinity))

	applyPVSource(res, localPV.Spec.PersistentVolumeSource.DeepCopy())

	return res
}

func applyPVSource(res *corev1apply.PersistentVolumeSpecApplyConfiguration, src *corev1.PersistentVolumeSource) {
	switch {
	case src.Local != nil:
		local := corev1apply.LocalVolumeSource().WithPath(src.Local.Path)
		if src.Local.FSType != nil {
			local.WithFSType(*src.Local.FSType)
		}
		res.WithLocal(local)
	case src.HostPath != nil:
		hostPath := corev1apply.HostPathVolumeSource().WithPath(src.HostPath.Path)
		if src.HostPath.Type != nil {
			hostPath.WithType(*src.HostPath.Type)
		}
		res.WithHostPath(hostPath)
	}
	// For local-storage PVCs we expect only Local or HostPath sources.
	// Other volume sources are intentionally ignored, as they are not supported
	// by this reflector.
}

func sanitizeNodeAffinity(affinity *corev1.VolumeNodeAffinity) *corev1apply.VolumeNodeAffinityApplyConfiguration {
	if affinity == nil || affinity.Required == nil {
		return dummyNodeAffinity()
	}

	var terms []*corev1apply.NodeSelectorTermApplyConfiguration
	for i := range affinity.Required.NodeSelectorTerms {
		term := &affinity.Required.NodeSelectorTerms[i]
		matchExpressions := filterNodeSelectorRequirements(term.MatchExpressions)
		matchFields := filterNodeSelectorRequirements(term.MatchFields)
		if len(matchExpressions) == 0 && len(matchFields) == 0 {
			continue
		}
		terms = append(terms, corev1apply.NodeSelectorTerm().
			WithMatchExpressions(matchExpressions...).
			WithMatchFields(matchFields...))
	}

	if len(terms) == 0 {
		return dummyNodeAffinity()
	}

	return corev1apply.VolumeNodeAffinity().
		WithRequired(corev1apply.NodeSelector().WithNodeSelectorTerms(terms...))
}

func filterNodeSelectorRequirements(reqs []corev1.NodeSelectorRequirement) []*corev1apply.NodeSelectorRequirementApplyConfiguration {
	var res []*corev1apply.NodeSelectorRequirementApplyConfiguration
	for i := range reqs {
		req := &reqs[i]
		if req.Key == consts.TypeLabel {
			// Drop Liqo virtual-node type selectors; they are meaningless in the remote cluster.
			continue
		}
		res = append(res, corev1apply.NodeSelectorRequirement().
			WithKey(req.Key).
			WithOperator(req.Operator).
			WithValues(req.Values...))
	}
	return res
}

func dummyNodeAffinity() *corev1apply.VolumeNodeAffinityApplyConfiguration {
	return corev1apply.VolumeNodeAffinity().
		WithRequired(corev1apply.NodeSelector().
			WithNodeSelectorTerms(corev1apply.NodeSelectorTerm().
				WithMatchExpressions(corev1apply.NodeSelectorRequirement().
					WithKey("kubernetes.io/os").
					WithOperator(corev1.NodeSelectorOpExists))))
}

func volumeResourceRequirements(resources corev1.VolumeResourceRequirements) *corev1apply.VolumeResourceRequirementsApplyConfiguration {
	return corev1apply.VolumeResourceRequirements().
		WithLimits(resources.Limits).
		WithRequests(resources.Requests)
}

func filterPVCAnnotations(annotations map[string]string) map[string]string {
	return maps.Filter(annotations, maps.FilterBlacklist(pvcAnnotationsBlacklist...))
}

func filterPVAnnotations(annotations map[string]string) map[string]string {
	return maps.Filter(annotations, maps.FilterBlacklist(pvAnnotationsBlacklist...))
}

const (
	annBindCompleted            = "pv.kubernetes.io/bind-completed"
	annBoundByController        = "pv.kubernetes.io/bound-by-controller"
	annMigratedTo               = "pv.kubernetes.io/migrated-to"
	annKubectlLastApplied       = "kubectl.kubernetes.io/last-applied-configuration"
	annStorageAlphaSelectedNode = "storage.alpha.kubernetes.io/selected-node"
	annGAStorageProvisioner     = "volume.kubernetes.io/storage-provisioner"
)

var pvcAnnotationsBlacklist = []string{
	annBindCompleted,
	annBoundByController,
	annDynamicallyProvisioned,
	annMigratedTo,
	annStorageProvisioner,
	annGAStorageProvisioner,
	annSelectedNode,
	annAlphaSelectedNode,
	annStorageAlphaSelectedNode,
	annKubectlLastApplied,
}

var pvAnnotationsBlacklist = []string{
	annBindCompleted,
	annBoundByController,
	annDynamicallyProvisioned,
	annMigratedTo,
	annStorageProvisioner,
	annGAStorageProvisioner,
	annSelectedNode,
	annAlphaSelectedNode,
	annStorageAlphaSelectedNode,
	annKubectlLastApplied,
}
