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

package shadowpod

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
	pod "github.com/liqotech/liqo/pkg/utils/pod"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// cluster-role
// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=shadowpods,verbs=get;list;watch
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch

// Validator is the handler used by the Validating Webhook to validate shadow pods.
type Validator struct {
	client                   client.Client
	PeeringCache             *peeringCache
	decoder                  *admission.Decoder
	enableResourceValidation bool
}

// NewValidator creates a new shadow pod validator.
func NewValidator(c client.Client, enableResourceValidation bool) *Validator {
	return &Validator{
		client:                   c,
		PeeringCache:             &peeringCache{ready: false},
		enableResourceValidation: enableResourceValidation,
	}
}

// Handle is the function in charge of handling the webhook validation request about the creation, update and deletion of shadowpods.
//
//nolint:gocritic // the signature of this method is imposed by controller runtime.
func (spv *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	klog.V(4).Infof("Operation: %s", req.Operation)

	if !spv.PeeringCache.ready && spv.enableResourceValidation {
		klog.Infof("Unable to process the request: Resource Validator cache not ready")
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("initialization in progress"))
	}

	switch req.Operation {
	case admissionv1.Create:
		return spv.HandleCreate(ctx, &req)
	case admissionv1.Delete:
		return spv.HandleDelete(ctx, &req)
	case admissionv1.Update:
		return spv.HandleUpdate(ctx, &req)
	default:
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unsupported operation %s", req.Operation))
	}
}

// HandleCreate is the function in charge of handling Creation requests.
func (spv *Validator) HandleCreate(ctx context.Context, req *admission.Request) admission.Response {
	shadowpod, err := spv.DecodeShadowPod(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding shadow pod: %v", err)
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed decoding of ShadowPod: %w", err))
	}

	// Check existence and get shadow pod origin Cluster ID label
	clusterID, found := shadowpod.Labels[forge.LiqoOriginClusterIDKey]
	if !found {
		klog.Warningf("Missing origin Cluster ID label on ShadowPod %q", shadowpod.Name)
		return admission.Denied("missing origin Cluster ID label")
	}

	klog.V(5).Infof("ShadowPod %s decoded: UID: %s - clusterID %s", shadowpod.Name, shadowpod.GetUID(), clusterID)

	code, err := spv.validateShadowPodClusterID(ctx, shadowpod.GetNamespace(), clusterID)
	if err != nil {
		if code == http.StatusBadRequest {
			return admission.Errored(code, err)
		}
		return admission.Denied(err.Error())
	}

	if !spv.enableResourceValidation {
		return admission.Allowed("")
	}

	// Check existence and get resource offer by Cluster ID label
	resourceoffer, err := liqogetters.GetResourceOfferByLabel(ctx, spv.client, corev1.NamespaceAll,
		liqolabels.LocalLabelSelectorForCluster(clusterID))
	if err != nil {
		newErr := fmt.Errorf("error getting resource offer by label: %w", err)
		klog.Error(newErr)
		return admission.Errored(http.StatusInternalServerError, newErr)
	}

	clusterName := retrieveClusterName(ctx, spv.client, clusterID)

	klog.V(5).Infof("ResourceOffer found for cluster %q with Quota %s",
		clusterName, quotaFormatter(resourceoffer.Spec.ResourceQuota.Hard))

	peeringInfo := spv.PeeringCache.getOrCreatePeeringInfo(discoveryv1alpha1.ClusterIdentity{
		ClusterID:   clusterID,
		ClusterName: clusterName,
	}, resourceoffer.Spec.ResourceQuota.Hard)

	err = peeringInfo.testAndUpdateCreation(ctx, spv.client, shadowpod, *req.DryRun)
	if err != nil {
		klog.Warning(err)
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

// HandleUpdate is the function in charge of handling Update requests.
func (spv *Validator) HandleUpdate(ctx context.Context, req *admission.Request) admission.Response {
	shadowpod, err := spv.DecodeShadowPod(req.Object)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed decoding of ShadowPod: %w", err))
	}

	oldShadowpod, err := spv.DecodeShadowPod(req.OldObject)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed decoding of ShadowPod: %w", err))
	}

	// Check existence and get shadow pod origin Cluster ID label
	clusterID, found := shadowpod.Labels[forge.LiqoOriginClusterIDKey]
	if !found {
		klog.Warningf("Missing origin Cluster ID label on ShadowPod %q", shadowpod.Name)
		return admission.Denied("missing origin Cluster ID label")
	}

	// Check existence and get old shadow pod origin Cluster ID label
	oldClusterID, found := oldShadowpod.Labels[forge.LiqoOriginClusterIDKey]
	if !found {
		klog.Warningf("Missing origin Cluster ID label on ShadowPod %q", oldShadowpod.Name)
		return admission.Denied("missing origin Cluster ID label")
	}

	if clusterID != oldClusterID {
		klog.Warningf("The Cluster ID label of the updated shadowpod %s is different from the old one %s", clusterID, oldClusterID)
		return admission.Denied("shadopow Cluster ID label is changed")
	}

	if pod.CheckShadowPodUpdate(&shadowpod.Spec.Pod, &oldShadowpod.Spec.Pod) {
		return admission.Allowed("")
	}

	return admission.Denied("")
}

// HandleDelete is the function in charge of handling Deletion requests.
func (spv *Validator) HandleDelete(ctx context.Context, req *admission.Request) admission.Response {
	shadowpod, err := spv.DecodeShadowPod(req.OldObject)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed decoding of ShadowPod: %w", err))
	}

	// Check existence and get shadow pod origin Cluster ID label
	clusterID, found := shadowpod.Labels[forge.LiqoOriginClusterIDKey]
	if !found {
		klog.Warningf("Missing origin Cluster ID label on ShadowPod %q", shadowpod.Name)
		return admission.Allowed("missing origin Cluster ID label")
	}

	klog.V(5).Infof("ShadowPod %s decoded: UID: %s - clusterID %s", shadowpod.Name, shadowpod.GetUID(), clusterID)
	if !spv.enableResourceValidation {
		return admission.Allowed("")
	}

	clusterName := retrieveClusterName(ctx, spv.client, clusterID)

	peeringInfo, found := spv.PeeringCache.getPeeringInfo(discoveryv1alpha1.ClusterIdentity{
		ClusterID:   clusterID,
		ClusterName: clusterName,
	})
	if !found {
		// If the PeeringInfo is not present in the Cache it means there are some cache consistency issues.
		// The next refreshing process will align this issue.
		// Anyway, the deletion process of shadowpods is always allowed.
		klog.Warningf("PeeringInfo not found in cache for cluster %q", clusterName)
		return admission.Allowed(fmt.Sprintf("Peering not found in cache for cluster %q", clusterName))
	}

	err = peeringInfo.updateDeletion(shadowpod, *req.DryRun)
	if err != nil {
		// The error could be generated by the absence of the ShadodPod Description in cache.
		// or for a UID mismatch. In both cases, the deletion is always allowed.
		// The next refreshing process will align this issue.
		klog.Warning(err)
		return admission.Allowed(err.Error())
	}

	return admission.Allowed("")
}

func (spv *Validator) validateShadowPodClusterID(ctx context.Context, ns, spClusterID string) (int32, error) {
	// Get ShadowPod Namespace
	namespace := &corev1.Namespace{}
	if err := spv.client.Get(ctx, client.ObjectKey{Name: ns}, namespace); err != nil {
		return http.StatusBadRequest, err
	}

	// Get Cluster ID origin label of the namespace
	nsClusterID, found := namespace.Labels[consts.RemoteClusterID]
	if !found || nsClusterID != spClusterID {
		klog.Warningf("Cluster ID %q of namespace %q does not match the ShadowPod Cluster ID %q", nsClusterID, namespace.Name, spClusterID)
		return http.StatusForbidden, fmt.Errorf("shadowpod Cluster ID label mismatch")
	}

	return http.StatusOK, nil
}

// DecodeShadowPod decodes a shadow pod from a given runtime object.
func (spv *Validator) DecodeShadowPod(obj runtime.RawExtension) (shadowpod *vkv1alpha1.ShadowPod, err error) {
	shadowpod = &vkv1alpha1.ShadowPod{}
	err = spv.decoder.DecodeRaw(obj, shadowpod)
	return
}

func (spv *Validator) getShadowPodListByClusterID(ctx context.Context,
	clusterID string) (shadowPodList *vkv1alpha1.ShadowPodList, err error) {
	shadowPodList = &vkv1alpha1.ShadowPodList{}
	err = spv.client.List(ctx, shadowPodList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{forge.LiqoOriginClusterIDKey: clusterID}),
	})
	return
}

// InjectDecoder injects the decoder.
func (spv *Validator) InjectDecoder(d *admission.Decoder) error {
	spv.decoder = d
	return nil
}
