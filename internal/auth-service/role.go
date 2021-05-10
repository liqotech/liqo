package auth_service

import (
	"context"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/liqotech/liqo/pkg/discovery"
)

func (authService *AuthServiceCtrl) createRole(remoteClusterId string, sa *v1.ServiceAccount) (*rbacv1.Role, error) {
	role := &rbacv1.Role{
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
			Labels: map[string]string{
				discovery.LiqoManagedLabel: "true",
				discovery.ClusterIdLabel:   remoteClusterId,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{v1.SchemeGroupVersion.Group},
				Resources: []string{"secrets"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups:     []string{v1.SchemeGroupVersion.Group},
				Resources:     []string{"secrets"},
				Verbs:         []string{"get", "delete"},
				ResourceNames: []string{remoteClusterId},
			},
		},
	}
	return authService.clientset.RbacV1().Roles(authService.namespace).Create(context.TODO(), role, metav1.CreateOptions{})
}
