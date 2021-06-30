package namespacemapctrltestutils

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const (
	roleBindingName = "role-binding"
	roleType        = "Role"
	roleName        = "fake"
)

// The remote namespace must have at least 2 roleBinding with the clastix label.

// GetRoleBindingForASpecificNamespace provides a roleBinding in the namespace passed as parameter.
// The name of the RoleBinding is associated to the index passed as second parameter.
func GetRoleBindingForASpecificNamespace(namespaceName, localClusterID string, index int) rbacv1.RoleBinding {
	return rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", roleBindingName, index),
			Namespace: namespaceName,
			Labels: map[string]string{
				liqoconst.RoleBindingLabelKey: fmt.Sprintf("%s-%s", liqoconst.RoleBindingLabelValuePrefix, localClusterID),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     roleType,
			Name:     roleName,
		},
	}
}
