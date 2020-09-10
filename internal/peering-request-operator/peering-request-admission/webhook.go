package peering_request_admission

import (
	"encoding/json"
	"fmt"
	discoveryv1alpha1 "github.com/liqotech/liqo/api/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/peering-request-operator"
	"github.com/liqotech/liqo/pkg/crdClient"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/apis/core/v1"
	"net/http"
	"os"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

type WebhookServer struct {
	Server *http.Server

	client    *crdClient.CRDClient
	Namespace string
}

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1beta1.AddToScheme(runtimeScheme)
	_ = v1.AddToScheme(runtimeScheme)
}

func (whsvr *WebhookServer) validate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {

	peerReq := discoveryv1alpha1.PeeringRequest{}

	if err := json.Unmarshal(ar.Request.Object.Raw, &peerReq); err != nil {
		klog.Error(err, err.Error())
		return &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	klog.Info("PeeringRequest " + peerReq.Name + " Received")

	conf, err := peering_request_operator.GetConfig(whsvr.client, whsvr.Namespace)
	if err != nil {
		os.Exit(1)
	}

	if conf.AllowAll {
		// allow every request
		klog.Info("PeeringRequest " + peerReq.Name + " Allowed")
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result:  nil,
		}
	} else {
		// TODO: apply policy to accept/reject peering requests
		klog.Info("PeeringRequest " + peerReq.Name + " Denied")
		return &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "Invalid token",
			},
		}
	}
}

func (whsvr *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		klog.Error(nil, "empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Error(nil, "Content-Type="+contentType+", expect application/json")
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		klog.Error(err, "Can't decode body: "+err.Error())
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = whsvr.validate(&ar)
	}

	admissionReview := v1beta1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
	}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		klog.Error(err, "Can't encode response: "+err.Error())
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	if _, err := w.Write(resp); err != nil {
		klog.Error(err, "Can't write response: "+err.Error())
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}
