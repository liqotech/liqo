package mutate

import (
	"encoding/json"
	"fmt"
	v1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
)

// Mutate mutates the object received via admReview and creates a response
// that embeds a patch to the received pod
func (s *MutationServer) Mutate(body []byte, verbose bool) ([]byte, error) {
	var err error
	if verbose {
		log.Printf("recv: %s\n", string(body)) // untested section
	}

	// unmarshal request into AdmissionReview struct
	admReview := v1beta1.AdmissionReview{}
	if err := json.Unmarshal(body, &admReview); err != nil {
		return nil, fmt.Errorf("unmarshaling request failed with %s", err)
	}

	var pod *corev1.Pod

	responseBody := []byte{}
	ar := admReview.Request
	resp := v1beta1.AdmissionResponse{}

	if ar != nil {

		// get the Pod object and unmarshal it into its struct, if we cannot, we might as well stop here
		if err := json.Unmarshal(ar.Object.Raw, &pod); err != nil {
			return nil, fmt.Errorf("unable unmarshal pod json object %v", err)
		}

		// set response options
		resp.Allowed = true
		resp.UID = ar.UID
		pT := v1beta1.PatchTypeJSONPatch
		resp.PatchType = &pT // it's annoying that this needs to be a pointer as you cannot give a pointer to a constant?

		resp.AuditAnnotations = map[string]string{
			"liqo": "this pod is allowed to run in liqo",
		}

		type patchType struct {
			Op    string              `json:"op"`
			Path  string              `json:"path"`
			Value []corev1.Toleration `json:"value"`
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
	}

	if verbose {
		log.Printf("resp: %s\n", string(responseBody))
	}

	return responseBody, nil
}
