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

package identitymanager

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

type tokenManager interface {
	start(ctx context.Context)
	getConfig(secret *v1.Secret, remoteCluster discoveryv1alpha1.ClusterIdentity) (*rest.Config, error)
}

type iamTokenManager struct {
	client                    kubernetes.Interface
	availableClusterIDSecrets map[string]types.NamespacedName
	availableTokenMutex       sync.Mutex

	tokenFiles map[string]string
}

func (tokMan *iamTokenManager) start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-time.NewTicker(10 * time.Minute).C:
				klog.V(4).Info("Refreshing tokens...")
				for remoteClusterID, namespacedName := range tokMan.availableClusterIDSecrets {
					if err := tokMan.refreshToken(ctx, discoveryv1alpha1.ClusterIdentity{
						ClusterID:   remoteClusterID,
						ClusterName: remoteClusterID,
					}, namespacedName); err != nil {
						klog.Error(err)
						continue
					}
				}
				klog.V(4).Info("Tokens refresh completed")
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (tokMan *iamTokenManager) refreshToken(ctx context.Context, remoteCluster discoveryv1alpha1.ClusterIdentity,
	namespacedName types.NamespacedName) error {
	secret, err := tokMan.client.CoreV1().Secrets(namespacedName.Namespace).Get(ctx, namespacedName.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("[%v] %v", remoteCluster.ClusterName, err)
		return err
	}

	tok, err := getIAMBearerToken(secret, remoteCluster)
	if err != nil {
		klog.Errorf("[%v] %v", remoteCluster.ClusterName, err)
		return err
	}

	if _, err = tokMan.storeToken(remoteCluster, tok); err != nil {
		klog.Errorf("[%v] %v", remoteCluster.ClusterName, err)
		return err
	}
	return nil
}

func (tokMan *iamTokenManager) getConfig(secret *v1.Secret, remoteCluster discoveryv1alpha1.ClusterIdentity) (*rest.Config, error) {
	tok, err := getIAMBearerToken(secret, remoteCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	clusterEndpoint, err := getValue(secret, APIServerURLSecretKey, remoteCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	ca, err := getValue(secret, apiServerCaSecretKey, remoteCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	filename, err := tokMan.storeToken(remoteCluster, tok)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	tokMan.addClusterID(remoteCluster, types.NamespacedName{
		Namespace: secret.Namespace,
		Name:      secret.Name,
	})

	var proxyURL *url.URL
	var proxyFunc func(*http.Request) (*url.URL, error)
	proxyConfig, ok := secret.Data[apiProxyURLSecretKey]
	if ok {
		proxyURL, err = url.Parse(string(proxyConfig))
		if err != nil {
			klog.Errorf("an error occurred while parsing proxy url %s from secret %v/%v: %s", proxyConfig, secret.Namespace, secret.Name, err)
			return nil, err
		}
		proxyFunc = func(request *http.Request) (*url.URL, error) {
			return proxyURL, nil
		}
	}

	// create the rest config
	return &rest.Config{
		Host:            string(clusterEndpoint),
		BearerTokenFile: filename,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: ca,
		},
		Proxy: proxyFunc,
	}, nil
}

func (tokMan *iamTokenManager) addClusterID(remoteCluster discoveryv1alpha1.ClusterIdentity, secret types.NamespacedName) {
	tokMan.availableTokenMutex.Lock()
	defer tokMan.availableTokenMutex.Unlock()
	tokMan.availableClusterIDSecrets[remoteCluster.ClusterID] = secret
}

func (tokMan *iamTokenManager) storeToken(remoteCluster discoveryv1alpha1.ClusterIdentity, tok *token.Token) (string, error) {
	var err error
	filename, found := tokMan.tokenFiles[remoteCluster.ClusterID]
	if found {
		_, err = os.Stat(filename)
	}

	if !found || os.IsNotExist(err) {
		file, err := os.CreateTemp("", "token")
		if err != nil {
			klog.Errorf("Error creating the authentication token tmp file: %v", err)
			return "", err
		}

		if err = file.Close(); err != nil {
			klog.Errorf("Error closing the authentication token tmp file: %v", err)
			return "", err
		}

		filename = file.Name()
		tokMan.tokenFiles[remoteCluster.ClusterID] = filename
	}

	err = os.WriteFile(filename, []byte(tok.Token), 0600)
	if err != nil {
		klog.Errorf("Error writing the authentication token tmp file: %v", err)
		return "", err
	}

	return filename, nil
}

func getIAMBearerToken(secret *v1.Secret, remoteCluster discoveryv1alpha1.ClusterIdentity) (*token.Token, error) {
	region, err := getValue(secret, awsRegionSecretKey, remoteCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	accessKeyID, err := getValue(secret, awsAccessKeyIDSecretKey, remoteCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	secretAccessKey, err := getValue(secret, awsSecretAccessKeySecretKey, remoteCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	// start a new AWS session
	sessRemote, err := session.NewSession(&aws.Config{
		Region: aws.String(string(region)),
		Credentials: credentials.NewStaticCredentialsFromCreds(credentials.Value{
			AccessKeyID:     string(accessKeyID),
			SecretAccessKey: string(secretAccessKey),
		}),
	})
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	eksClusterID, err := getValue(secret, awsEKSClusterIDSecretKey, remoteCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	// set token options
	opts := &token.GetTokenOptions{
		ClusterID: string(eksClusterID),
		Session:   sessRemote,
	}

	gen, err := token.NewGenerator(true, false)
	if err != nil {
		return nil, err
	}

	// get a new bearer token
	tok, err := gen.GetWithOptions(opts)
	if err != nil {
		return nil, err
	}

	return &tok, nil
}

func getValue(secret *v1.Secret, key string, remoteCluster discoveryv1alpha1.ClusterIdentity) ([]byte, error) {
	value, ok := secret.Data[key]
	if !ok {
		klog.Errorf("key %v not found in secret %v/%v", key, secret.Namespace, secret.Name)
		err := kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, remoteCluster.ClusterID)
		return nil, err
	}
	return value, nil
}
