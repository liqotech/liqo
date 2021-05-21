package forge

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/vkMachinery"
)

// CreateVkDeployment creates the deployment for a virtual-kubelet.
func CreateVkDeployment(adv *advtypes.Advertisement, vkName, vkNamespace, vkImage, initVKImage, nodeName, homeClusterId string) (*appsv1.Deployment, error) {
	vkLabels := ForgeVKLabels(adv)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vkName,
			Namespace: vkNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: vkLabels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: vkLabels,
				},
				Spec: forgeVKPodSpec(vkName, vkNamespace, homeClusterId, adv, initVKImage, nodeName, vkImage),
			},
		},
	}, nil
}

func ForgeVKLabels(adv *advtypes.Advertisement) map[string]string {
	kubeletDynamicLabels := map[string]string{
		"liqo.io/cluster-id": adv.Spec.ClusterId,
	}
	return merge(vkMachinery.KubeletBaseLabels, kubeletDynamicLabels)
}

func merge(m map[string]string, ms ...map[string]string) map[string]string {
	for _, s := range ms {
		for k, v := range s {
			m[k] = v
		}
	}
	return m
}

func ForgeVKClusterRoleBinding(name string, kubeletNamespace string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", APIGroup: "", Name: name, Namespace: kubeletNamespace},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "liqo-virtual-kubelet-local",
		},
	}
}

func ForgeVKServiceAccount(name string, kubeletNamespace string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: kubeletNamespace,
		},
	}
}
