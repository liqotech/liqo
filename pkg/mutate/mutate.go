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

package mutate

import (
	"encoding/json"
	"fmt"

	"gomodules.xyz/jsonpatch/v2"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// cluster-role
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=get;list;update;patch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespaceoffloadings,verbs=get;list;watch
//role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=create;get;list;watch

// Mutate mutates the object received via admReview and creates a response
// that embeds a patch to the received pod.
func (s *MutationServer) Mutate(body []byte) ([]byte, error) {
	var err error

	// Unmarshal request into AdmissionReview struct.
	admReview := admissionv1beta1.AdmissionReview{}
	if err = json.Unmarshal(body, &admReview); err != nil {
		return nil, fmt.Errorf("unmarshaling request failed with %s", err)
	}

	var pod *corev1.Pod

	responseBody := []byte{}
	admissionReviewRequest := admReview.Request
	resp := admissionv1beta1.AdmissionResponse{}

	if admissionReviewRequest == nil {
		return responseBody, fmt.Errorf("received admissionReview with empty request")
	}

	// Get the Pod object and unmarshal it into its struct, if we cannot, we might as well stop here
	if err = json.Unmarshal(admissionReviewRequest.Object.Raw, &pod); err != nil {
		return nil, fmt.Errorf("unable unmarshal pod json object %v", err)
	}

	// set response options
	resp.Allowed = true
	resp.UID = admissionReviewRequest.UID
	patchStrategy := admissionv1beta1.PatchTypeJSONPatch
	resp.PatchType = &patchStrategy

	resp.AuditAnnotations = map[string]string{
		"liqo": "this pod is allowed to run in liqo",
	}

	original, err := json.Marshal(pod)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	// Get the NamespaceOffloading associated with the pod Namespace. If there is no NamespaceOffloading for that
	// Namespace, it is an error the liqo.io/scheduling label shouldn't be on this namespace.
	namespaceOffloading := &offv1alpha1.NamespaceOffloading{}
	if err = s.webhookClient.Get(s.ctx, types.NamespacedName{
		Namespace: admissionReviewRequest.Namespace,
		Name:      liqoconst.DefaultNamespaceOffloadingName,
	}, namespaceOffloading); err != nil {
		return nil, fmt.Errorf("%w -> unable to get the NamespaceOffloading for the Namespace: %s", err, pod.Namespace)
	}

	klog.V(5).Infof("The namespace '%s' has a NamespaceOffloading resource", pod.Namespace)
	if err = mutatePod(namespaceOffloading, pod); err != nil {
		return nil, err
	}

	target, err := json.Marshal(pod)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	ops, err := jsonpatch.CreatePatch(original, target)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	if resp.Patch, err = json.Marshal(ops); err != nil {
		klog.Error(err)
		return nil, err
	}

	resp.Result = &metav1.Status{
		Status: "Success",
	}

	admReview.Response = &resp
	responseBody, err = json.Marshal(admReview)
	if err != nil {
		return nil, err
	}

	// If the object name is empty the server will generate it after the mutation.
	if admissionReviewRequest.Name == "" {
		admissionReviewRequest.Name = "Name omitted"
	}
	klog.Infof("Namespace: %s  Pod name: %s  -> patched", admissionReviewRequest.Namespace, admissionReviewRequest.Name)
	klog.V(8).Infof("response: %s", string(responseBody))

	return responseBody, nil
}
