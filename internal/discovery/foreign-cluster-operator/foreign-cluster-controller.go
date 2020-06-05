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
	"github.com/liqoTech/liqo/internal/discovery/kubeconfig"
	"github.com/liqoTech/liqo/pkg/clusterID"
	v1 "github.com/liqoTech/liqo/pkg/discovery/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
)

// ForeignClusterReconciler reconciles a ForeignCluster object
type ForeignClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	Namespace       string
	client          *kubernetes.Clientset
	discoveryClient *v1.DiscoveryV1Client
	clusterID       *clusterID.ClusterID
}

// +kubebuilder:rbac:groups=discovery.drone.com,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.drone.com,resources=foreignclusters/status,verbs=get;update;patch

func (r *ForeignClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("foreigncluster", req.NamespacedName)

	fc, err := r.discoveryClient.ForeignClusters().Get(req.Name, metav1.GetOptions{})
	if err != nil {
		// TODO: has been removed
		return ctrl.Result{}, err
	}

	if fc.Spec.Federate && !fc.Status.Federated {
		// create FederationRequest
		foreignConfig, err := fc.GetConfig()
		if err != nil {
			r.Log.Error(err, err.Error())
			return ctrl.Result{}, err
		}
		_, err = r.createFederationRequestIfNotExists(req.Name, fc, foreignConfig)
		if err != nil {
			r.Log.Error(err, err.Error())
			return ctrl.Result{}, err
		}
		fc.Status.Federated = true
		_, err = r.discoveryClient.ForeignClusters().Update(fc, metav1.UpdateOptions{})
		if err != nil {
			r.Log.Error(err, err.Error())
			return ctrl.Result{}, err
		}
	}
	if !fc.Spec.Federate && fc.Status.Federated {
		// TODO: delete federation request
		// this cluster can only delete own federation requests
	}

	return ctrl.Result{}, nil
}

func (r *ForeignClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1.ForeignCluster{}).
		Complete(r)
}

func (r *ForeignClusterReconciler) createFederationRequestIfNotExists(clusterID string, owner *discoveryv1.ForeignCluster, foreignConfig *rest.Config) (*discoveryv1.FederationRequest, error) {
	foreignDiscoveryClient, _ := v1.NewForConfig(foreignConfig)

	// get config to send to foreign cluster
	fConfig, err := r.getForeignConfig(clusterID, owner)
	if err != nil {
		return nil, err
	}

	localClusterID := r.clusterID.GetClusterID()

	fr, err := foreignDiscoveryClient.FederationRequests().Get(localClusterID, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// does not exist
			fr := discoveryv1.FederationRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: localClusterID,
				},
				Spec: discoveryv1.FederationRequestSpec{
					ClusterID:  localClusterID,
					KubeConfig: fConfig,
				},
			}
			return foreignDiscoveryClient.FederationRequests().Create(&fr)
		}
		// other errors
		return nil, err
	}
	// already exists
	return fr, nil
}

// this function return a kube-config file to send to foreign cluster and crate everything needed for it
func (r *ForeignClusterReconciler) getForeignConfig(clusterID string, owner *discoveryv1.ForeignCluster) (string, error) {
	_, err := r.createClusterRoleIfNotExists(clusterID, owner)
	if err != nil {
		return "", err
	}
	sa, err := r.createServiceAccountIfNotExists(clusterID, owner)
	if err != nil {
		return "", err
	}
	_, err = r.createClusterRoleBindingIfNotExists(clusterID, owner)
	if err != nil {
		return "", err
	}
	// check if ServiceAccount already has a secret, wait if not
	if len(sa.Secrets) == 0 {
		wa, err := r.client.CoreV1().ServiceAccounts(r.Namespace).Watch(metav1.ListOptions{
			FieldSelector: "metadata.name=" + clusterID,
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
		wa.Stop()
	}
	cnf, err := kubeconfig.CreateKubeConfig(clusterID, r.Namespace)
	return b64.StdEncoding.EncodeToString([]byte(cnf)), err
}

func (r *ForeignClusterReconciler) createClusterRoleIfNotExists(clusterID string, owner *discoveryv1.ForeignCluster) (*rbacv1.ClusterRole, error) {
	role, err := r.client.RbacV1().ClusterRoles().Get(clusterID, metav1.GetOptions{})
	if err != nil {
		// does not exist
		role = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: owner.APIVersion,
						Kind:       owner.Kind,
						Name:       owner.Name,
						UID:        owner.UID,
					},
				},
			},
			Rules: []rbacv1.PolicyRule{
				// TODO: set correct access to create advertisements
				{
					Verbs:     []string{"get", "list", "create", "delete", "watch"},
					APIGroups: []string{"protocol.drone.com"},
					Resources: []string{"advertisements"},
				},
			},
		}
		return r.client.RbacV1().ClusterRoles().Create(role)
	} else {
		return role, nil
	}
}

func (r *ForeignClusterReconciler) createServiceAccountIfNotExists(clusterID string, owner *discoveryv1.ForeignCluster) (*apiv1.ServiceAccount, error) {
	sa, err := r.client.CoreV1().ServiceAccounts(r.Namespace).Get(clusterID, metav1.GetOptions{})
	if err != nil {
		// does not exist
		sa = &apiv1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: owner.APIVersion,
						Kind:       owner.Kind,
						Name:       owner.Name,
						UID:        owner.UID,
					},
				},
			},
		}
		return r.client.CoreV1().ServiceAccounts(r.Namespace).Create(sa)
	} else {
		return sa, nil
	}
}

func (r *ForeignClusterReconciler) createClusterRoleBindingIfNotExists(clusterID string, owner *discoveryv1.ForeignCluster) (*rbacv1.ClusterRoleBinding, error) {
	rb, err := r.client.RbacV1().ClusterRoleBindings().Get(clusterID, metav1.GetOptions{})
	if err != nil {
		// does not exist
		rb = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: owner.APIVersion,
						Kind:       owner.Kind,
						Name:       owner.Name,
						UID:        owner.UID,
					},
				},
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      clusterID,
					Namespace: r.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     clusterID,
			},
		}
		return r.client.RbacV1().ClusterRoleBindings().Create(rb)
	} else {
		return rb, nil
	}
}
