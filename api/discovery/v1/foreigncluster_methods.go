package v1

import (
	b64 "encoding/base64"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func (fc *ForeignCluster) GetConfig() (*rest.Config, error) {
	bytes, err := b64.StdEncoding.DecodeString(fc.Spec.KubeConfig)
	if err != nil {
		return nil, err
	}
	kubeconfig := func() (*clientcmdapi.Config, error) {
		return clientcmd.Load(bytes)
	}
	return clientcmd.BuildConfigFromKubeconfigGetter("", kubeconfig)
}
