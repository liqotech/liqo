package auth_service

import (
	"context"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"net/http"
	"strings"
)

func isNoContent(err error) bool {
	switch t := err.(type) {
	case *kerrors.StatusError:
		return t.ErrStatus.Code == http.StatusNoContent
	}
	return false
}

func (authService *AuthServiceCtrl) getServiceAccountCompleted(remoteClusterId string) (sa *v1.ServiceAccount, err error) {
	err = retry.OnError(
		retry.DefaultBackoff,
		func(err error) bool {
			err2 := authService.saInformer.GetStore().Resync()
			if err2 != nil {
				klog.Error(err)
				return false
			}
			return kerrors.IsNotFound(err)
		},
		func() error {
			sa, err = authService.getServiceAccount(remoteClusterId)
			return err
		},
	)

	err = retry.OnError(retry.DefaultBackoff, isNoContent, func() error {
		sa, err = authService.getServiceAccount(remoteClusterId)
		if err != nil {
			return err
		}

		if len(sa.Secrets) == 0 {
			return &kerrors.StatusError{ErrStatus: metav1.Status{
				Status: metav1.StatusFailure,
				Code:   http.StatusNoContent,
				Reason: metav1.StatusReasonNotFound,
			}}
		}

		return nil
	})
	return sa, err
}

func (authService *AuthServiceCtrl) getServiceAccount(remoteClusterId string) (*v1.ServiceAccount, error) {
	tmp, exists, err := authService.saInformer.GetStore().GetByKey(strings.Join([]string{authService.namespace, remoteClusterId}, "/"))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, kerrors.NewNotFound(schema.GroupResource{
			Resource: "serviceaccounts",
		}, remoteClusterId)
	}
	sa, ok := tmp.(*v1.ServiceAccount)
	if !ok {
		return nil, kerrors.NewNotFound(schema.GroupResource{
			Resource: "serviceaccounts",
		}, remoteClusterId)
	}
	return sa, nil
}

func (authService *AuthServiceCtrl) createServiceAccount(remoteClusterId string) (*v1.ServiceAccount, error) {
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: remoteClusterId,
		},
	}
	return authService.clientset.CoreV1().ServiceAccounts(authService.namespace).Create(context.TODO(), sa, metav1.CreateOptions{})
}
