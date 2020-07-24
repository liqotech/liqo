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
	goerrors "errors"
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/discovery"
	"github.com/liqoTech/liqo/internal/discovery/kubeconfig"
	"github.com/liqoTech/liqo/pkg/clusterID"
	"github.com/liqoTech/liqo/pkg/crdClient"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

// ForeignClusterReconciler reconciles a ForeignCluster object
type ForeignClusterReconciler struct {
	Scheme *runtime.Scheme

	Namespace           string
	crdClient           *crdClient.CRDClient
	advertisementClient *crdClient.CRDClient
	clusterID           *clusterID.ClusterID
	RequeueAfter        time.Duration

	DiscoveryCtrl *discovery.DiscoveryCtrl

	// testing
	ForeignConfig *rest.Config
}

// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;update;patch

func (r *ForeignClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()

	klog.Info("Reconciling ForeignCluster " + req.Name)

	tmp, err := r.crdClient.Resource("foreignclusters").Get(req.Name, metav1.GetOptions{})
	if err != nil {
		// TODO: has been removed
		return ctrl.Result{}, nil
	}
	fc, ok := tmp.(*discoveryv1.ForeignCluster)
	if !ok {
		klog.Error("created object is not a ForeignCluster")
		return ctrl.Result{}, goerrors.New("created object is not a ForeignCluster")
	}

	requireUpdate := false

	// if it has no discovery type label, add it
	if fc.ObjectMeta.Labels == nil {
		fc.ObjectMeta.Labels = map[string]string{}
	}
	if fc.ObjectMeta.Labels["discovery-type"] == "" {
		fc.ObjectMeta.Labels["discovery-type"] = string(fc.Spec.DiscoveryType)
		requireUpdate = true
	}

	// check if linked advertisement exists
	if fc.Status.Advertisement != nil {
		_, err := r.advertisementClient.Resource("advertisements").Get(fc.Status.Advertisement.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fc.Status.Advertisement = nil
			fc.Status.Joined = false
			fc.Status.PeeringRequestName = ""
			fc.Spec.Join = false
			requireUpdate = true
		}
	}

	if fc.Status.CaDataRef == nil {
		klog.Info("Get CA Data")
		err = fc.LoadForeignCA(r.crdClient.Client(), r.Namespace, r.ForeignConfig)
		if err != nil {
			klog.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		_, err = r.crdClient.Resource("foreignclusters").Update(fc.Name, fc, metav1.UpdateOptions{})
		if err != nil {
			klog.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		klog.Info("CA Data successfully loaded for ForeignCluster " + fc.Name)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, nil
	}

	foreignConfig, err := fc.GetConfig(r.crdClient.Client())
	if err != nil {
		klog.Error(err, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, err
	}
	foreignConfig.GroupVersion = &discoveryv1.GroupVersion
	foreignDiscoveryClient, err := crdClient.NewFromConfig(foreignConfig)
	if err != nil {
		klog.Error(err, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, err
	}

	// if join is required (both automatically or by user) and status is not set to joined
	// create new peering request
	if fc.Spec.Join && !fc.Status.Joined {
		// create PeeringRequest
		pr, err := r.createPeeringRequestIfNotExists(req.Name, fc, foreignDiscoveryClient)
		if err != nil {
			klog.Error(err, err.Error())
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
			klog.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		// local advertisement has to be removed
		err = r.deleteAdvertisement(fc)
		if err != nil {
			klog.Error(err, err.Error())
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
		_, err = r.crdClient.Resource("foreignclusters").Update(fc.Name, fc, metav1.UpdateOptions{})
		if err != nil {
			klog.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		klog.Info("ForeignCluster " + fc.Name + " successfully reconciled")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, nil
	}

	// check if peering request really exists on foreign cluster
	if fc.Spec.Join && fc.Status.Joined {
		_, err = r.checkJoined(fc, foreignDiscoveryClient)
		if err != nil {
			klog.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
	}

	klog.Info("ForeignCluster " + fc.Name + " successfully reconciled")
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: r.RequeueAfter,
	}, nil
}

func (r *ForeignClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1.ForeignCluster{}).Owns(&protocolv1.Advertisement{}).
		Complete(r)
}

func (r *ForeignClusterReconciler) checkJoined(fc *discoveryv1.ForeignCluster, foreignDiscoveryClient *crdClient.CRDClient) (*discoveryv1.ForeignCluster, error) {
	_, err := foreignDiscoveryClient.Resource("peeringrequests").Get(fc.Status.PeeringRequestName, metav1.GetOptions{})
	if err != nil {
		fc.Status.Joined = false
		fc.Status.PeeringRequestName = ""
		tmp, err := r.crdClient.Resource("foreignclusters").Update(fc.Name, fc, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		var ok bool
		fc, ok = tmp.(*discoveryv1.ForeignCluster)
		if !ok {
			return nil, goerrors.New("updated object is not a ForeignCluster")
		}
	}
	return fc, nil
}

func (r *ForeignClusterReconciler) createPeeringRequestIfNotExists(clusterID string, owner *discoveryv1.ForeignCluster, foreignClient *crdClient.CRDClient) (*discoveryv1.PeeringRequest, error) {
	// get config to send to foreign cluster
	fConfig, err := r.getForeignConfig(clusterID, owner)
	if err != nil {
		return nil, err
	}

	localClusterID := r.clusterID.GetClusterID()

	tmp, err := foreignClient.Resource("peeringrequests").Get(localClusterID, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// does not exist
			pr := &discoveryv1.PeeringRequest{
				TypeMeta: metav1.TypeMeta{
					Kind:       "PeeringRequest",
					APIVersion: "discovery.liqo.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: localClusterID,
				},
				Spec: discoveryv1.PeeringRequestSpec{
					ClusterID:     localClusterID,
					Namespace:     r.Namespace,
					KubeConfigRef: nil,
				},
			}
			tmp, err = foreignClient.Resource("peeringrequests").Create(pr, metav1.CreateOptions{})
			if err != nil {
				return nil, err
			}
			var ok bool
			pr, ok = tmp.(*discoveryv1.PeeringRequest)
			if !ok {
				return nil, goerrors.New("created object is not a ForeignCluster")
			}
			secret := &apiv1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pr-" + clusterID,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "discovery.liqo.io/v1",
							Kind:       "PeeringRequest",
							Name:       pr.Name,
							UID:        pr.UID,
						},
					},
				},
				StringData: map[string]string{
					"kubeconfig": fConfig,
				},
			}
			secret, err := foreignClient.Client().CoreV1().Secrets(r.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
			if err != nil {
				return nil, err
			}
			pr.Spec.KubeConfigRef = &apiv1.ObjectReference{
				Kind:       "Secret",
				Namespace:  secret.Namespace,
				Name:       secret.Name,
				UID:        secret.UID,
				APIVersion: "v1",
			}
			pr.TypeMeta.Kind = "PeeringRequest"
			pr.TypeMeta.APIVersion = "discovery.liqo.io/v1"
			tmp, err = foreignClient.Resource("peeringrequests").Update(pr.Name, pr, metav1.UpdateOptions{})
			if err != nil {
				return nil, err
			}
			pr, ok = tmp.(*discoveryv1.PeeringRequest)
			if !ok {
				return nil, goerrors.New("created object is not a ForeignCluster")
			}
			return pr, nil
		}
		// other errors
		return nil, err
	}
	// already exists
	pr, ok := tmp.(*discoveryv1.PeeringRequest)
	if !ok {
		return nil, goerrors.New("retrieved object is not a PeeringRequest")
	}
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
		wa, err := r.crdClient.Client().CoreV1().ServiceAccounts(r.Namespace).Watch(context.TODO(), metav1.ListOptions{
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
	cnf, err := kubeconfig.CreateKubeConfig(r.crdClient.Client(), clusterID, r.Namespace)
	return cnf, err
}

func (r *ForeignClusterReconciler) createClusterRoleIfNotExists(clusterID string, owner *discoveryv1.ForeignCluster) (*rbacv1.ClusterRole, error) {
	role, err := r.crdClient.Client().RbacV1().ClusterRoles().Get(context.TODO(), clusterID, metav1.GetOptions{})
	if err != nil {
		// does not exist
		role = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "ForeignCluster",
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
					Resources: []string{"advertisements", "advertisements/status", "secrets"},
				},
			},
		}
		return r.crdClient.Client().RbacV1().ClusterRoles().Create(context.TODO(), role, metav1.CreateOptions{})
	} else {
		return role, nil
	}
}

func (r *ForeignClusterReconciler) createServiceAccountIfNotExists(clusterID string, owner *discoveryv1.ForeignCluster) (*apiv1.ServiceAccount, error) {
	sa, err := r.crdClient.Client().CoreV1().ServiceAccounts(r.Namespace).Get(context.TODO(), clusterID, metav1.GetOptions{})
	if err != nil {
		// does not exist
		sa = &apiv1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "ForeignCluster",
						Name:       owner.Name,
						UID:        owner.UID,
					},
				},
			},
		}
		return r.crdClient.Client().CoreV1().ServiceAccounts(r.Namespace).Create(context.TODO(), sa, metav1.CreateOptions{})
	} else {
		return sa, nil
	}
}

func (r *ForeignClusterReconciler) createClusterRoleBindingIfNotExists(clusterID string, owner *discoveryv1.ForeignCluster) (*rbacv1.ClusterRoleBinding, error) {
	rb, err := r.crdClient.Client().RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterID, metav1.GetOptions{})
	if err != nil {
		// does not exist
		rb = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "ForeignCluster",
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
		return r.crdClient.Client().RbacV1().ClusterRoleBindings().Create(context.TODO(), rb, metav1.CreateOptions{})
	} else {
		return rb, nil
	}
}

func (r *ForeignClusterReconciler) deleteAdvertisement(fc *discoveryv1.ForeignCluster) error {
	return fc.DeleteAdvertisement(r.advertisementClient)
}

func (r *ForeignClusterReconciler) deletePeeringRequest(foreignClient *crdClient.CRDClient, fc *discoveryv1.ForeignCluster) error {
	return foreignClient.Resource("peeringrequests").Delete(fc.Status.PeeringRequestName, metav1.DeleteOptions{})
}
