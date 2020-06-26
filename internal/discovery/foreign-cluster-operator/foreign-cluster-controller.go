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
	"github.com/go-logr/logr"
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
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
	"time"
)

// ForeignClusterReconciler reconciles a ForeignCluster object
type ForeignClusterReconciler struct {
	Log    logr.Logger
	Scheme *runtime.Scheme

	Namespace       string
	client          *kubernetes.Clientset
	discoveryClient *v1.DiscoveryV1Client
	clusterID       *clusterID.ClusterID
	RequeueAfter    time.Duration

	// testing
	ForeignConfig *rest.Config
}

// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;update;patch

func (r *ForeignClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("foreigncluster", req.NamespacedName)

	fc, err := r.discoveryClient.ForeignClusters().Get(req.Name, metav1.GetOptions{})
	if err != nil {
		// TODO: has been removed
		return ctrl.Result{}, nil
	}

	if fc.Status.CaDataRef == nil {
		r.Log.Info("Get CA Data")
		err = fc.LoadForeignCA(r.client, r.Namespace, r.ForeignConfig)
		if err != nil {
			r.Log.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		_, err = r.discoveryClient.ForeignClusters().Update(fc, metav1.UpdateOptions{})
		if err != nil {
			r.Log.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, nil
	}

	foreignConfig, err := fc.GetConfig(r.client)
	if err != nil {
		r.Log.Error(err, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, err
	}
	foreignK8sClient, err := kubernetes.NewForConfig(foreignConfig)
	if err != nil {
		r.Log.Error(err, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, err
	}
	foreignDiscoveryClient, err := v1.NewForConfig(foreignConfig)
	if err != nil {
		r.Log.Error(err, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, err
	}

	requireUpdate := false

	// if join is required (both automatically or by user) and status is not set to joined
	// create new peering request
	if fc.Spec.Join && !fc.Status.Joined {
		// create PeeringRequest
		pr, err := r.createPeeringRequestIfNotExists(req.Name, fc, foreignDiscoveryClient, foreignK8sClient)
		if err != nil {
			r.Log.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		fc.Status.Joined = true
		fc.Status.PeeringRequestName = pr.Name
		requireUpdate = true
	}

	// if join is no more required and status is set to joined
	// delete peering request
	if !fc.Spec.Join && fc.Status.Joined {
		// peering request has to be removed
		err := r.deletePeeringRequest(foreignDiscoveryClient, fc)
		if err != nil {
			r.Log.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		fc.Status.Joined = false
		fc.Status.PeeringRequestName = ""
		requireUpdate = true
	}

	if requireUpdate {
		_, err = r.discoveryClient.ForeignClusters().Update(fc, metav1.UpdateOptions{})
		if err != nil {
			r.Log.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, nil
	}

	// check if peering request really exists on foreign cluster
	if fc.Spec.Join && fc.Status.Joined {
		_, err = r.checkJoined(fc, foreignDiscoveryClient)
		if err != nil {
			r.Log.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
	}

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: r.RequeueAfter,
	}, nil
}

func (r *ForeignClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1.ForeignCluster{}).
		Complete(r)
}

func (r *ForeignClusterReconciler) checkJoined(fc *discoveryv1.ForeignCluster, foreignDiscoveryClient *v1.DiscoveryV1Client) (*discoveryv1.ForeignCluster, error) {
	_, err := foreignDiscoveryClient.PeeringRequests().Get(fc.Status.PeeringRequestName, metav1.GetOptions{})
	if err != nil {
		fc.Status.Joined = false
		fc.Status.PeeringRequestName = ""
		fc, err = r.discoveryClient.ForeignClusters().Update(fc, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
	}
	return fc, nil
}

func (r *ForeignClusterReconciler) createPeeringRequestIfNotExists(clusterID string, owner *discoveryv1.ForeignCluster, foreignClient *v1.DiscoveryV1Client, foreignK8sClient *kubernetes.Clientset) (*discoveryv1.PeeringRequest, error) {
	// get config to send to foreign cluster
	fConfig, err := r.getForeignConfig(clusterID, owner)
	if err != nil {
		return nil, err
	}

	localClusterID := r.clusterID.GetClusterID()

	pr, err := foreignClient.PeeringRequests().Get(localClusterID, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// does not exist
			pr := &discoveryv1.PeeringRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: localClusterID,
				},
				Spec: discoveryv1.PeeringRequestSpec{
					ClusterID:     localClusterID,
					Namespace:     r.Namespace,
					KubeConfigRef: nil,
				},
			}
			pr, err = foreignClient.PeeringRequests().Create(pr)
			if err != nil {
				return nil, err
			}
			secret := &apiv1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pr-" + clusterID,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: pr.APIVersion,
							Kind:       pr.APIVersion,
							Name:       pr.Name,
							UID:        pr.UID,
						},
					},
				},
				StringData: map[string]string{
					"kubeconfig": fConfig,
				},
			}
			secret, err := foreignK8sClient.CoreV1().Secrets(r.Namespace).Create(secret)
			if err != nil {
				return nil, err
			}
			pr.Spec.KubeConfigRef = &apiv1.ObjectReference{
				Kind:       secret.Kind,
				Namespace:  secret.Namespace,
				Name:       secret.Name,
				UID:        secret.UID,
				APIVersion: secret.APIVersion,
			}
			return foreignClient.PeeringRequests().Update(pr, metav1.UpdateOptions{})
		}
		// other errors
		return nil, err
	}
	// already exists
	return pr, nil
}

// this function return a kube-config file to send to foreign cluster and crate everything needed for it
func (r *ForeignClusterReconciler) getForeignConfig(clusterID string, owner *discoveryv1.ForeignCluster) (string, error) {
	if r.ForeignConfig != nil {
		return r.ForeignConfig.String(), nil
	}
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
	return cnf, err
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
					Verbs:     []string{"get", "list", "create", "update", "delete", "watch"},
					APIGroups: []string{"protocol.liqo.io", ""},
					Resources: []string{"advertisements", "secrets"},
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

func (r *ForeignClusterReconciler) deletePeeringRequest(foreignClient *v1.DiscoveryV1Client, fc *discoveryv1.ForeignCluster) error {
	return foreignClient.PeeringRequests().Delete(fc.Status.PeeringRequestName, metav1.DeleteOptions{})
}
