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

package shadowpod

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/getters"
	pod "github.com/liqotech/liqo/pkg/utils/pod"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// cluster-role
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=shadowpods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=quotas,verbs=get;list;watch

// Validator is the handler used by the Validating Webhook to validate shadow pods.
type Validator struct {
	client                   client.Client
	PeeringCache             *peeringCache
	decoder                  admission.Decoder
	enableResourceValidation bool
}

// NewValidator creates a new shadow pod validator.
func NewValidator(c client.Client, enableResourceValidation bool) *Validator {
	return &Validator{
		client:                   c,
		PeeringCache:             &peeringCache{ready: false},
		enableResourceValidation: enableResourceValidation,
		decoder:                  admission.NewDecoder(runtime.NewScheme()),
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
	shadowpod, err := decodeShadowPod(spv.decoder, req.Object)
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

	if shadowpod.Labels == nil {
		return admission.Denied("missing creator label")
	}
	creatorName, found := shadowpod.Labels[consts.CreatorLabelKey]
	if !found {
		return admission.Denied("missing creator label")
	}

	quota, err := getters.GetQuotaByUser(ctx, spv.client, creatorName)
	if err != nil {
		klog.Warningf("Failed getting quota for user %s: %v", creatorName, err)
		return admission.Denied("failed getting quota")
	}

	klog.V(5).Infof("Quota found for user %q with %s",
		creatorName, quotaFormatter(quota.Spec.Resources))

	if quota.Spec.Cordoned != nil && *quota.Spec.Cordoned {
		klog.Warningf("User %q is cordoned", creatorName)
		return admission.Denied("user is cordoned")
	}

	peeringInfo := spv.PeeringCache.getOrCreatePeeringInfo(creatorName, quota.Spec.Resources)

	err = peeringInfo.testAndUpdateCreation(ctx, spv.client, shadowpod, quota.Spec.LimitsEnforcement, *req.DryRun)
	if err != nil {
		klog.Warning(err)
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

// HandleUpdate is the function in charge of handling Update requests.
func (spv *Validator) HandleUpdate(ctx context.Context, req *admission.Request) admission.Response {
	shadowpod, err := decodeShadowPod(spv.decoder, req.Object)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed decoding of ShadowPod: %w", err))
	}

	oldShadowpod, err := decodeShadowPod(spv.decoder, req.OldObject)
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
	shadowpod, err := decodeShadowPod(spv.decoder, req.OldObject)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed decoding of ShadowPod: %w", err))
	}

	creatorName, found := shadowpod.Labels[consts.CreatorLabelKey]
	if !found {
		klog.Warningf("Missing creator label on ShadowPod %q", shadowpod.Name)
		return admission.Denied("missing creator label")
	}

	if !spv.enableResourceValidation {
		return admission.Allowed("")
	}

	// TODO: use the creator label to get the user
	peeringInfo, found := spv.PeeringCache.getPeeringInfo(creatorName)
	if !found {
		// If the PeeringInfo is not present in the Cache it means there are some cache consistency issues.
		// The next refreshing process will align this issue.
		// Anyway, the deletion process of shadowpods is always allowed.
		klog.Warningf("PeeringInfo not found in cache for user %q", creatorName)
		return admission.Allowed(fmt.Sprintf("Peering not found in cache for user %q", creatorName))
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

// decodeShadowPod decodes a shadow pod from a given runtime object.
func decodeShadowPod(decoder admission.Decoder, obj runtime.RawExtension) (shadowpod *offloadingv1beta1.ShadowPod, err error) {
	shadowpod = &offloadingv1beta1.ShadowPod{}
	err = decoder.DecodeRaw(obj, shadowpod)
	return
}

var _ webhook.AdmissionHandler = &Mutator{}

// Mutator is the handler used by the Mutating Webhook to mutate shadow pods.
type Mutator struct {
	client  client.Client
	decoder admission.Decoder
}

// NewMutator creates a new shadow pod mutator.
func NewMutator(c client.Client) *webhook.Admission {
	return &webhook.Admission{Handler: &Mutator{
		client:  c,
		decoder: admission.NewDecoder(runtime.NewScheme()),
	}}
}

// Handle is the function in charge of handling the webhook mutating request about the creation, update and deletion of shadowpods.
//
//nolint:gocritic // the signature of this method is imposed by controller runtime.
func (spm *Mutator) Handle(_ context.Context, req admission.Request) admission.Response {
	klog.V(4).Infof("Operation: %s", req.Operation)

	switch req.Operation {
	case admissionv1.Create:
		return spm.HandleCreate(&req)
	case admissionv1.Delete:
		return spm.HandleDelete()
	case admissionv1.Update:
		return spm.HandleUpdate(&req)
	default:
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unsupported operation %s", req.Operation))
	}
}

// HandleCreate is the function in charge of handling Creation requests.
func (spm *Mutator) HandleCreate(req *admission.Request) admission.Response {
	sp, err := decodeShadowPod(spm.decoder, req.Object)
	if err != nil {
		klog.Errorf("Failed decoding shadow pod: %v", err)
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed decoding of ShadowPod: %w", err))
	}

	creatorName, err := extractCreatorInfo(&req.UserInfo)
	if err != nil {
		klog.Warningf("Failed extracting creator info: %v", err)
		return admission.Denied(err.Error())
	}

	if sp.Labels == nil {
		sp.Labels = map[string]string{}
	}
	sp.Labels[consts.CreatorLabelKey] = creatorName

	marshaledShadowPod, err := json.Marshal(sp)
	if err != nil {
		klog.Errorf("Failed marshaling ShadowPod object: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledShadowPod)
}

// HandleUpdate is the function in charge of handling Update requests.
func (spm *Mutator) HandleUpdate(req *admission.Request) admission.Response {
	oldSp, err := decodeShadowPod(spm.decoder, req.OldObject)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed decoding of ShadowPod: %w", err))
	}

	sp, err := decodeShadowPod(spm.decoder, req.Object)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed decoding of ShadowPod: %w", err))
	}

	if oldSp.Labels == nil {
		return admission.Denied("missing creator name")
	}
	oldCreatorName, ok := oldSp.Labels[consts.CreatorLabelKey]
	if !ok {
		return admission.Denied("missing creator name")
	}

	creatorName, err := extractCreatorInfo(&req.UserInfo)
	if err != nil {
		klog.Warningf("Failed extracting creator info: %v", err)
		return admission.Denied(err.Error())
	}

	if oldCreatorName != creatorName {
		return admission.Denied("creator name cannot be modified")
	}

	if sp.Labels == nil {
		sp.Labels = map[string]string{}
	}
	sp.Labels[consts.CreatorLabelKey] = creatorName

	marshaledShadowPod, err := json.Marshal(sp)
	if err != nil {
		klog.Errorf("Failed marshaling ShadowPod object: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledShadowPod)
}

// HandleDelete is the function in charge of handling Deletion requests.
func (spm *Mutator) HandleDelete() admission.Response {
	return admission.Allowed("")
}

func extractCreatorInfo(userInfo *authenticationv1.UserInfo) (creatorName string, err error) {
	creatorName = userInfo.Username
	if creatorName == "" {
		return "", fmt.Errorf("missing creator name")
	}

	return
}
