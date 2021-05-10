package mutate

import (
	"encoding/json"
	"fmt"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

type patchType struct {
	Op    string              `json:"op"`
	Path  string              `json:"path"`
	Value []corev1.Toleration `json:"value"`
}

// cluster-role
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=get;list;update;patch
// role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=create;get;list;watch

// Mutate mutates the object received via admReview and creates a response
// that embeds a patch to the received pod
func (s *MutationServer) Mutate(body []byte) ([]byte, error) {
	var err error

	// unmarshal request into AdmissionReview struct
	admReview := admissionv1beta1.AdmissionReview{}
	if err := json.Unmarshal(body, &admReview); err != nil {
		return nil, fmt.Errorf("unmarshaling request failed with %s", err)
	}

	var pod *corev1.Pod

	responseBody := []byte{}
	admissionReviewRequest := admReview.Request
	resp := admissionv1beta1.AdmissionResponse{}

	if admissionReviewRequest == nil {
		return responseBody, fmt.Errorf("received admissionReview with empty request")
	}

	// get the Pod object and unmarshal it into its struct, if we cannot, we might as well stop here
	if err := json.Unmarshal(admissionReviewRequest.Object.Raw, &pod); err != nil {
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

	tolerations := append(pod.Spec.Tolerations, corev1.Toleration{
		Key:      "virtual-node.liqo.io/not-allowed",
		Operator: "Exists",
		Effect:   "NoExecute",
	})

	patch := []patchType{
		{
			Op:    "add",
			Path:  "/spec/tolerations",
			Value: tolerations,
		},
	}
	if resp.Patch, err = json.Marshal(patch); err != nil {
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

	klog.Infof("pod %s/%s patched", pod.Namespace, pod.Name)
	klog.V(8).Infof("response: %s", string(responseBody))

	return responseBody, nil
}
