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

package identitymanager

import (
	"context"
	"encoding/base64"
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

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

type tokenManager interface {
	start(ctx context.Context)
	mutateConfig(secret *v1.Secret, remoteCluster liqov1beta1.ClusterID, cnf *rest.Config) (*rest.Config, error)
}

var _ tokenManager = &iamTokenManager{}

type iamTokenManager struct {
	client                    kubernetes.Interface
	availableClusterIDSecrets map[liqov1beta1.ClusterID]types.NamespacedName
	availableTokenMutex       sync.Mutex

	// TODO: the key cannot be the clusterID
	tokenFiles map[liqov1beta1.ClusterID]string
}

func (tokMan *iamTokenManager) start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-time.NewTicker(10 * time.Minute).C:
				klog.V(4).Info("Refreshing tokens...")
				for remoteClusterID, namespacedName := range tokMan.availableClusterIDSecrets {
					if err := tokMan.refreshToken(ctx, remoteClusterID, namespacedName); err != nil {
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

func (tokMan *iamTokenManager) refreshToken(ctx context.Context, remoteCluster liqov1beta1.ClusterID,
	namespacedName types.NamespacedName) error {
	secret, err := tokMan.client.CoreV1().Secrets(namespacedName.Namespace).Get(ctx, namespacedName.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("[%v] %v", remoteCluster, err)
		return err
	}

	tok, err := getIAMBearerToken(secret, remoteCluster)
	if err != nil {
		klog.Errorf("[%v] %v", remoteCluster, err)
		return err
	}

	if _, err = tokMan.storeToken(remoteCluster, tok); err != nil {
		klog.Errorf("[%v] %v", remoteCluster, err)
		return err
	}
	return nil
}

func (tokMan *iamTokenManager) mutateConfig(secret *v1.Secret, remoteCluster liqov1beta1.ClusterID,
	cnf *rest.Config) (*rest.Config, error) {
	tok, err := getIAMBearerToken(secret, remoteCluster)
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

	cnf.BearerTokenFile = filename
	cnf.BearerToken = ""

	cert, err := base64.StdEncoding.DecodeString(string(cnf.CAData))
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	cnf.TLSClientConfig.CAData = cert

	return cnf, nil
}

func (tokMan *iamTokenManager) addClusterID(remoteCluster liqov1beta1.ClusterID, secret types.NamespacedName) {
	tokMan.availableTokenMutex.Lock()
	defer tokMan.availableTokenMutex.Unlock()
	tokMan.availableClusterIDSecrets[remoteCluster] = secret
}

func (tokMan *iamTokenManager) storeToken(remoteCluster liqov1beta1.ClusterID, tok *token.Token) (string, error) {
	var err error
	filename, found := tokMan.tokenFiles[remoteCluster]
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
		tokMan.tokenFiles[remoteCluster] = filename
	}

	err = os.WriteFile(filename, []byte(tok.Token), 0o600)
	if err != nil {
		klog.Errorf("Error writing the authentication token tmp file: %v", err)
		return "", err
	}

	return filename, nil
}

func getIAMBearerToken(secret *v1.Secret, remoteCluster liqov1beta1.ClusterID) (*token.Token, error) {
	region, err := getValue(secret, AwsRegionSecretKey, remoteCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	accessKeyID, err := getValue(secret, AwsAccessKeyIDSecretKey, remoteCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	secretAccessKey, err := getValue(secret, AwsSecretAccessKeySecretKey, remoteCluster)
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

	eksClusterID, err := getValue(secret, AwsEKSClusterIDSecretKey, remoteCluster)
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

func getValue(secret *v1.Secret, key string, remoteCluster liqov1beta1.ClusterID) ([]byte, error) {
	value, ok := secret.Data[key]
	if !ok {
		klog.Errorf("key %v not found in secret %v/%v", key, secret.Namespace, secret.Name)
		err := kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, string(remoteCluster))
		return nil, err
	}
	return value, nil
}
