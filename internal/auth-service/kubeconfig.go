package auth_service

import (
	v1 "k8s.io/api/core/v1"

	"github.com/liqotech/liqo/pkg/kubeconfig"
)

// this function creates a kube-config file for a specified ServiceAccount.
func (authService *AuthServiceCtrl) createKubeConfig(serviceAccount *v1.ServiceAccount) (string, error) {
	return kubeconfig.CreateKubeConfig(authService, authService.clientset, serviceAccount.Name, serviceAccount.Namespace)
}
