package authservice

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

const (
	authTokenSecretName = "auth-token"
)

type tokenManager interface {
	getToken() (string, error)
	createToken() error
}

func (authService *Controller) getToken() (string, error) {
	obj, exists, err := authService.secretInformer.GetStore().GetByKey(
		strings.Join([]string{
			authService.namespace,
			authTokenSecretName}, "/"))
	if err != nil {
		klog.Error(err)
		return "", err
	} else if !exists {
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, authTokenSecretName)
		klog.Error(err)
		return "", err
	}

	secret, ok := obj.(*v1.Secret)
	if !ok {
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, authTokenSecretName)
		klog.Error(err)
		return "", err
	}

	return authService.getTokenFromSecret(secret.DeepCopy())
}

func (authService *Controller) createToken() error {
	_, exists, _ := authService.secretInformer.GetStore().GetByKey(
		strings.Join([]string{
			authService.namespace,
			authTokenSecretName}, "/"))
	if !exists {
		token, err := generateToken()
		if err != nil {
			return err
		}

		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: authTokenSecretName,
			},
			StringData: map[string]string{
				"token": token,
			},
		}
		_, err = authService.clientset.CoreV1().Secrets(
			authService.namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil && !kerrors.IsAlreadyExists(err) {
			klog.Error(err)
			return err
		}
	}
	return nil
}

func (authService *Controller) getTokenFromSecret(secret *v1.Secret) (string, error) {
	v, ok := secret.Data["token"]
	if !ok {
		// TODO: specialise secret type
		err := errors.New("invalid secret")
		klog.Error(err)
		return "", err
	}
	return string(v), nil
}

func generateToken() (string, error) {
	b := make([]byte, 64)
	_, err := rand.Read(b)
	if err != nil {
		klog.Error(err)
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
