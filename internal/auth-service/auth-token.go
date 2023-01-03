// Copyright 2019-2023 The Liqo Authors
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

package authservice

import (
	"context"

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
		authService.namespace + "/" + auth.TokenSecretName)
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
		authService.namespace + "/" + auth.TokenSecretName)
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
