/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package foreign_cluster_operator

import (
	"context"
	b64 "encoding/base64"
	"github.com/go-logr/logr"
	"github.com/netgroup-polito/dronev2/internal/discovery"
	"github.com/netgroup-polito/dronev2/internal/discovery/kubeconfig"
	v1 "github.com/netgroup-polito/dronev2/pkg/discovery/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1 "github.com/netgroup-polito/dronev2/api/discovery/v1"
)

// ForeignClusterReconciler reconciles a ForeignCluster object
type ForeignClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=discovery.drone.com,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.drone.com,resources=foreignclusters/status,verbs=get;update;patch

func (r *ForeignClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("foreigncluster", req.NamespacedName)

	discoveryClient, _ := discovery.NewDiscoveryClient()
	fc, err := discoveryClient.ForeignClusters(req.Namespace).Get(req.Name, metav1.GetOptions{})
	if err != nil {
		// TODO: has been removed
		return ctrl.Result{}, err
	}

	if fc.Spec.Federate {
		foreignConfig, _ := fc.GetConfig()
		_, _ = CreateFederationRequestIfNotExists(req.Name, fc, foreignConfig)
	}

	return ctrl.Result{}, nil
}

func (r *ForeignClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1.ForeignCluster{}).
		Complete(r)
}

func CreateFederationRequestIfNotExists(clusterID string, owner *discoveryv1.ForeignCluster, foreignConfig *rest.Config) (*discoveryv1.FederationRequest, error) {
	discoveryClient, _ := v1.NewForConfig(foreignConfig)

	// get config to sent to foreign cluster
	fConfig, err := GetForeignConfig(clusterID, owner)

	// TODO: get correct name
	fr, err := discoveryClient.FederationRequests(apiv1.NamespaceDefault).Get("test", metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		// does not exist
		fr := discoveryv1.FederationRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: discoveryv1.FederationRequestSpec{
				KubeConfig: fConfig,
			},
		}
		return discoveryClient.FederationRequests(apiv1.NamespaceDefault).Create(&fr)
	} else if err == nil {
		// already exists
		return fr, nil
	} else {
		return nil, err
	}
}

func GetForeignConfig(clusterID string, owner *discoveryv1.ForeignCluster) (string, error) {
	clientK8s, _ := discovery.NewK8sClient()
	_, err := CreateRoleIfNotExists(clientK8s, clusterID, owner)
	if err != nil {
		return "", err
	}
	sa, err := CreateServiceAccountIfNotExists(clientK8s, clusterID, owner)
	if err != nil {
		return "", err
	}
	_, err = CreateRoleBindingIfNotExists(clientK8s, clusterID, owner)
	if err != nil {
		return "", err
	}
	// check if ServiceAccount already has a secret, wait if not
	if len(sa.Secrets) == 0 {
		wa, err := clientK8s.CoreV1().ServiceAccounts(apiv1.NamespaceDefault).Watch(metav1.ListOptions{
			Watch: true,
		})
		if err != nil {
			return "", err
		}
		ch := wa.ResultChan()
		for s := range ch {
			_sa := s.Object.(*apiv1.ServiceAccount)
			if _sa.Name == sa.Name && len(_sa.Secrets) > 0 {
				break
			}
		}
	}
	cnf, err := kubeconfig.CreateKubeConfig(clusterID)
	return b64.StdEncoding.EncodeToString([]byte(cnf)), err
}

func CreateRoleIfNotExists(clientK8s *kubernetes.Clientset, clusterID string, owner *discoveryv1.ForeignCluster) (*rbacv1.Role, error) {
	role, err := clientK8s.RbacV1().Roles(apiv1.NamespaceDefault).Get(clusterID, metav1.GetOptions{})
	if err != nil {
		// does not exist
		t := true
		f := false
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         owner.APIVersion,
						Kind:               owner.Kind,
						Name:               owner.Name,
						UID:                owner.UID,
						Controller:         &t,
						BlockOwnerDeletion: &f,
					},
				},
			},
			Rules: []rbacv1.PolicyRule{
				// TODO: set correct access to create advertisements
				{
					Verbs:     []string{"get", "list"},
					APIGroups: []string{""},
					Resources: []string{"pods"},
				},
			},
		}
		return clientK8s.RbacV1().Roles(apiv1.NamespaceDefault).Create(role)
	} else {
		return role, nil
	}
}

func CreateServiceAccountIfNotExists(clientK8s *kubernetes.Clientset, clusterID string, owner *discoveryv1.ForeignCluster) (*apiv1.ServiceAccount, error) {
	sa, err := clientK8s.CoreV1().ServiceAccounts(apiv1.NamespaceDefault).Get(clusterID, metav1.GetOptions{})
	if err != nil {
		// does not exist
		t := true
		f := false
		sa = &apiv1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         owner.APIVersion,
						Kind:               owner.Kind,
						Name:               owner.Name,
						UID:                owner.UID,
						Controller:         &t,
						BlockOwnerDeletion: &f,
					},
				},
			},
		}
		return clientK8s.CoreV1().ServiceAccounts(apiv1.NamespaceDefault).Create(sa)
	} else {
		return sa, nil
	}
}

func CreateRoleBindingIfNotExists(clientK8s *kubernetes.Clientset, clusterID string, owner *discoveryv1.ForeignCluster) (*rbacv1.RoleBinding, error) {
	rb, err := clientK8s.RbacV1().RoleBindings(apiv1.NamespaceDefault).Get(clusterID, metav1.GetOptions{})
	if err != nil {
		// does not exist
		t := true
		f := false
		rb = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         owner.APIVersion,
						Kind:               owner.Kind,
						Name:               owner.Name,
						UID:                owner.UID,
						Controller:         &t,
						BlockOwnerDeletion: &f,
					},
				},
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      clusterID,
					Namespace: apiv1.NamespaceDefault,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     clusterID,
			},
		}
		return clientK8s.RbacV1().RoleBindings(apiv1.NamespaceDefault).Create(rb)
	} else {
		return rb, nil
	}
}
