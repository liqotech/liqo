package auth

import (
	"encoding/base64"
	"io/ioutil"

	"github.com/liqotech/liqo/pkg/utils"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/kubeconfig"
)

// CertificateIdentityResponse is the response on a certificate identity request.
type CertificateIdentityResponse struct {
	Namespace    string `json:"namespace"`
	Certificate  string `json:"certificate"`
	APIServerURL string `json:"apiServerUrl"`
	APIServerCA  string `json:"apiServerCA,omitempty"`
}

// NewCertificateIdentityResponse makes a new CertificateIdentityResponse.
func NewCertificateIdentityResponse(
	namespace string, certificate []byte, apiServerConfigProvider utils.ApiServerConfigProvider,
	clientset kubernetes.Interface, restConfig *rest.Config) (*CertificateIdentityResponse, error) {
	apiServerURL, err := kubeconfig.GetApiServerURL(apiServerConfigProvider, clientset)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	var apiServerCa string
	if apiServerConfigProvider.GetAPIServerConfig().TrustedCA {
		apiServerCa = ""
	} else {
		apiServerCa, err = getAPIServerCA(restConfig)
		if err != nil {
			klog.Error(err)
			return nil, err
		}
	}

	return &CertificateIdentityResponse{
		Namespace:    namespace,
		Certificate:  base64.StdEncoding.EncodeToString(certificate),
		APIServerURL: apiServerURL,
		APIServerCA:  apiServerCa,
	}, nil
}

// getAPIServerCA retrieves the ApiServerCA.
// It can take it from the CAData in the restConfig, or reading it from the CAFile.
func getAPIServerCA(restConfig *rest.Config) (string, error) {
	if restConfig.CAData != nil && len(restConfig.CAData) > 0 {
		// CAData available in the restConfig, encode and return it.
		return base64.StdEncoding.EncodeToString(restConfig.CAData), nil
	} else if restConfig.CAFile != "" {
		// CAData is not available, read it from the CAFile.
		dat, err := ioutil.ReadFile(restConfig.CAFile)
		if err != nil {
			klog.Error(err)
			return "", err
		}
		return base64.StdEncoding.EncodeToString(dat), nil
	} else {
		klog.Warning("empty CA data")
		return "", nil
	}
}
