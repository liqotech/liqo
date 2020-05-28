package v1

import (
	b64 "encoding/base64"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func (fr *FederationRequest) GetConfig() (*rest.Config, error) {
	bytes, err := b64.StdEncoding.DecodeString(fr.Spec.KubeConfig)
	if err != nil {
		return nil, err
	}
	kubeconfig := func() (*clientcmdapi.Config, error) {
		return clientcmd.Load(bytes)
	}
	return clientcmd.BuildConfigFromKubeconfigGetter("", kubeconfig)
}
