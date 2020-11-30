package auth_service

import (
	"context"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (authService *AuthServiceCtrl) createClusterRole(remoteClusterId string, sa *v1.ServiceAccount) (*rbacv1.ClusterRole, error) {
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: sa.Name,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
					Name:       sa.Name,
					UID:        sa.UID,
				},
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{discoveryv1alpha1.GroupVersion.Group},
				Resources: []string{"peeringrequests"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups:     []string{discoveryv1alpha1.GroupVersion.Group},
				Resources:     []string{"peeringrequests"},
				Verbs:         []string{"get", "delete", "update"},
				ResourceNames: []string{remoteClusterId},
			},
		},
	}
	return authService.clientset.RbacV1().ClusterRoles().Create(context.TODO(), role, metav1.CreateOptions{})
}
