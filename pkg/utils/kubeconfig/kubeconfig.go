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

package kubeconfig

import (
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"
)

// GenerateKubeconfig generates a kubeconfig file with the provided user, cluster, server, and certificate data.
func GenerateKubeconfig(user, cluster, server string, ca, clientCertificate, clientKey []byte, proxyURL, namespace *string) ([]byte, error) {
	clusters := make(map[string]*clientcmdapi.Cluster)
	clusters[cluster] = &clientcmdapi.Cluster{
		Server:                   server,
		CertificateAuthorityData: ca,
		ProxyURL:                 ptr.Deref(proxyURL, ""),
	}

	contexts := make(map[string]*clientcmdapi.Context)
	contexts["default-context"] = &clientcmdapi.Context{
		Cluster:   cluster,
		Namespace: ptr.Deref(namespace, ""),
		AuthInfo:  user,
	}

	authinfos := make(map[string]*clientcmdapi.AuthInfo)
	authinfos[user] = &clientcmdapi.AuthInfo{
		ClientKeyData:         clientKey,
		ClientCertificateData: clientCertificate,
	}

	clientConfig := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: "default-context",
		AuthInfos:      authinfos,
	}

	return clientcmd.Write(clientConfig)
}
