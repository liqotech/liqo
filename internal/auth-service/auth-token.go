package authservice

import (
	"context"
	"strings"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/auth"
)

type tokenManager interface {
	getToken() (string, error)
	createToken() error
}

func (authService *Controller) getToken() (string, error) {
	obj, exists, err := authService.secretInformer.GetStore().GetByKey(
		strings.Join([]string{
			authService.namespace,
			auth.TokenSecretName}, "/"))
	if err != nil {
		klog.Error(err)
		return "", err
	} else if !exists {
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, auth.TokenSecretName)
		klog.Error(err)
		return "", err
	}

	secret, ok := obj.(*v1.Secret)
	if !ok {
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, auth.TokenSecretName)
		klog.Error(err)
		return "", err
	}

	return auth.GetTokenFromSecret(secret.DeepCopy())
}

func (authService *Controller) createToken() error {
	_, exists, _ := authService.secretInformer.GetStore().GetByKey(
		strings.Join([]string{
			authService.namespace,
			auth.TokenSecretName}, "/"))
	if !exists {
		token, err := auth.GenerateToken()
		if err != nil {
			return err
		}

		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: auth.TokenSecretName,
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
