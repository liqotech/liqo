package identitymanager

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
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
)

type tokenManager interface {
	start(ctx context.Context)
	getConfig(secret *v1.Secret, remoteClusterID string) (*rest.Config, error)
}

const tokenDir = "token"

type iamTokenManager struct {
	client                    kubernetes.Interface
	availableClusterIDSecrets map[string]types.NamespacedName
	availableTokenMutex       sync.Mutex
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

func (tokMan *iamTokenManager) refreshToken(ctx context.Context, remoteClusterID string, namespacedName types.NamespacedName) error {
	secret, err := tokMan.client.CoreV1().Secrets(namespacedName.Namespace).Get(ctx, namespacedName.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("[%v] %v", remoteClusterID, err)
		return err
	}

	tok, err := getIAMBearerToken(secret, remoteClusterID)
	if err != nil {
		klog.Errorf("[%v] %v", remoteClusterID, err)
		return err
	}

	if _, err = tokMan.storeToken(remoteClusterID, tok); err != nil {
		klog.Errorf("[%v] %v", remoteClusterID, err)
		return err
	}
	return nil
}

func (tokMan *iamTokenManager) getConfig(secret *v1.Secret, remoteClusterID string) (*rest.Config, error) {
	tok, err := getIAMBearerToken(secret, remoteClusterID)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	clusterEndpoint, err := getValue(secret, apiServerURLSecretKey, remoteClusterID)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	ca, err := getValue(secret, apiServerCaSecretKey, remoteClusterID)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	filename, err := tokMan.storeToken(remoteClusterID, tok)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	tokMan.addClusterID(remoteClusterID, types.NamespacedName{
		Namespace: secret.Namespace,
		Name:      secret.Name,
	})

	// create the rest config
	return &rest.Config{
		Host:            string(clusterEndpoint),
		BearerTokenFile: filename,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: ca,
		},
	}, nil
}

func (tokMan *iamTokenManager) addClusterID(remoteClusterID string, secret types.NamespacedName) {
	tokMan.availableTokenMutex.Lock()
	defer tokMan.availableTokenMutex.Unlock()
	tokMan.availableClusterIDSecrets[remoteClusterID] = secret
}

func (tokMan *iamTokenManager) storeToken(remoteClusterID string, tok *token.Token) (string, error) {
	_, err := os.Stat(tokenDir)
	if os.IsNotExist(err) {
		if err = os.Mkdir(tokenDir, 0750); err != nil {
			klog.Error(err)
			return "", err
		}
	}

	filename := filepath.Join(tokenDir, remoteClusterID)
	err = ioutil.WriteFile(filename, []byte(tok.Token), 0600)
	if err != nil {
		klog.Error(err)
		return "", err
	}

	return filename, nil
}

func getIAMBearerToken(secret *v1.Secret, remoteClusterID string) (*token.Token, error) {
	region, err := getValue(secret, awsRegionSecretKey, remoteClusterID)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	accessKeyID, err := getValue(secret, awsAccessKeyIDSecretKey, remoteClusterID)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	secretAccessKey, err := getValue(secret, awsSecretAccessKeySecretKey, remoteClusterID)
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

	eksClusterID, err := getValue(secret, awsEKSClusterIDSecretKey, remoteClusterID)
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

func getValue(secret *v1.Secret, key, remoteClusterID string) ([]byte, error) {
	value, ok := secret.Data[key]
	if !ok {
		klog.Errorf("key %v not found in secret %v/%v", key, secret.Namespace, secret.Name)
		err := kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, remoteClusterID)
		return nil, err
	}
	return value, nil
}
