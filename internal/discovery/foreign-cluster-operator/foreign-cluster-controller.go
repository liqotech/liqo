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
	discoveryv1alpha1 "github.com/liqotech/liqo/api/discovery/v1alpha1"
	advtypes "github.com/liqotech/liqo/api/sharing/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery"
	"github.com/liqotech/liqo/internal/discovery/kubeconfig"
	"github.com/liqotech/liqo/pkg/clusterID"
	"github.com/liqotech/liqo/pkg/crdClient"
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
	fc, ok := tmp.(*discoveryv1alpha1.ForeignCluster)
	if !ok {
		klog.Error("created object is not a ForeignCluster")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, goerrors.New("created object is not a ForeignCluster")
	}

	requireUpdate := false

	// if it has no discovery type label, add it
	if fc.ObjectMeta.Labels == nil {
		fc.ObjectMeta.Labels = map[string]string{}
	}
	if fc.ObjectMeta.Labels["discovery-type"] == "" || fc.ObjectMeta.Labels["discovery-type"] != string(fc.Spec.DiscoveryType) {
		fc.ObjectMeta.Labels["discovery-type"] = string(fc.Spec.DiscoveryType)
		requireUpdate = true
	}
	// set cluster-id label to easy retrieve ForeignClusters by ClusterId,
	// if it is added manually, the name maybe not coincide with ClusterId
	if fc.ObjectMeta.Labels["cluster-id"] == "" {
		fc.ObjectMeta.Labels["cluster-id"] = fc.Spec.ClusterID
		requireUpdate = true
	}

	// check if linked advertisement exists
	if fc.Status.Outgoing.Advertisement != nil {
		tmp, err = r.advertisementClient.Resource("advertisements").Get(fc.Status.Outgoing.Advertisement.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fc.Status.Outgoing.Advertisement = nil
			fc.Status.Outgoing.AvailableIdentity = false
			fc.Status.Outgoing.IdentityRef = nil
			fc.Status.Outgoing.AdvertisementStatus = ""
			fc.Status.Outgoing.Joined = false
			fc.Status.Outgoing.RemotePeeringRequestName = ""
			fc.Spec.Join = false
			requireUpdate = true
		} else if err == nil {
			// check if kubeconfig secret exists
			adv, ok := tmp.(*advtypes.Advertisement)
			if !ok {
				err = goerrors.New("retrieved object is not an Advertisement")
				klog.Error(err)
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: r.RequeueAfter,
				}, err
			}
			if adv.Spec.KubeConfigRef.Name != "" && adv.Spec.KubeConfigRef.Namespace != "" {
				_, err = r.crdClient.Client().CoreV1().Secrets(adv.Spec.KubeConfigRef.Namespace).Get(context.TODO(), adv.Spec.KubeConfigRef.Name, metav1.GetOptions{})
				available := err == nil
				if fc.Status.Outgoing.AvailableIdentity != available {
					fc.Status.Outgoing.AvailableIdentity = available
					if available {
						fc.Status.Outgoing.IdentityRef = &apiv1.ObjectReference{
							Kind:       "Secret",
							Namespace:  adv.Spec.KubeConfigRef.Namespace,
							Name:       adv.Spec.KubeConfigRef.Name,
							APIVersion: "v1",
						}
					}
					requireUpdate = true
				}
			}

			// update advertisement status
			status := adv.Status.AdvertisementStatus
			if status != fc.Status.Outgoing.AdvertisementStatus {
				fc.Status.Outgoing.AdvertisementStatus = status
				requireUpdate = true
			}
		}
	}

	// check if linked peeringRequest exists
	if fc.Status.Incoming.PeeringRequest != nil {
		tmp, err = r.crdClient.Resource("peeringrequests").Get(fc.Status.Incoming.PeeringRequest.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fc.Status.Incoming.PeeringRequest = nil
			fc.Status.Incoming.AvailableIdentity = false
			fc.Status.Incoming.IdentityRef = nil
			fc.Status.Incoming.AdvertisementStatus = ""
			fc.Status.Incoming.Joined = false
			requireUpdate = true
		} else if err == nil {
			pr, ok := tmp.(*discoveryv1alpha1.PeeringRequest)
			if !ok {
				err = goerrors.New("retrieved object is not a PeeringRequest")
				klog.Error(err)
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: r.RequeueAfter,
				}, err
			}

			// PeeringRequest exists, set flag to true
			fc.Status.Incoming.Joined = true
			requireUpdate = true

			// check if kubeconfig secret exists
			if pr.Spec.KubeConfigRef != nil && pr.Spec.KubeConfigRef.Name != "" && pr.Spec.KubeConfigRef.Namespace != "" {
				_, err = r.crdClient.Client().CoreV1().Secrets(pr.Spec.KubeConfigRef.Namespace).Get(context.TODO(), pr.Spec.KubeConfigRef.Name, metav1.GetOptions{})
				available := err == nil
				if fc.Status.Incoming.AvailableIdentity != available {
					fc.Status.Incoming.AvailableIdentity = available
					if available {
						fc.Status.Incoming.IdentityRef = pr.Spec.KubeConfigRef
					}
					requireUpdate = true
				}
			}

			// update advertisement status
			status := pr.Status.AdvertisementStatus
			if status != fc.Status.Incoming.AdvertisementStatus {
				fc.Status.Incoming.AdvertisementStatus = status
				requireUpdate = true
			}
		}
	}

	// if it has been discovered thanks to incoming peeringRequest and it has no active connections, delete it
	if fc.Spec.DiscoveryType == discoveryv1alpha1.IncomingPeeringDiscovery && fc.Status.Incoming.PeeringRequest == nil && fc.Status.Outgoing.Advertisement == nil {
		err = r.crdClient.Resource("foreignclusters").Delete(fc.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		klog.Info(fc.Name + " deleted, discovery type " + string(fc.Spec.DiscoveryType) + " has no active connections")
		klog.Info("ForeignCluster " + fc.Name + " successfully reconciled")
		return ctrl.Result{}, nil
	}

	if fc.Status.Outgoing.CaDataRef == nil && fc.Spec.AllowUntrustedCA {
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
		// delete reference, in this way at next iteration it will be reloaded
		fc.Status.Outgoing.CaDataRef = nil
		_, err2 := r.crdClient.Resource("foreignclusters").Update(fc.Name, fc, metav1.UpdateOptions{})
		if err2 != nil {
			klog.Error(err2, err2.Error())
		}
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, err
	}
	foreignConfig.GroupVersion = &discoveryv1alpha1.GroupVersion
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
	if fc.Spec.Join && !fc.Status.Outgoing.Joined {
		// create PeeringRequest
		klog.Info("Creating PeeringRequest")
		pr, err := r.createPeeringRequestIfNotExists(req.Name, fc, foreignDiscoveryClient)
		if err != nil {
			klog.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		fc.Status.Outgoing.Joined = true
		fc.Status.Outgoing.RemotePeeringRequestName = pr.Name
		requireUpdate = true
	}

	// if join is no more required and status is set to joined
	// delete peering request
	if !fc.Spec.Join && fc.Status.Outgoing.Joined {
		// peering request has to be removed
		klog.Info("Deleting PeeringRequest")
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
		fc.Status.Outgoing.Joined = false
		fc.Status.Outgoing.RemotePeeringRequestName = ""
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
	if fc.Spec.Join && fc.Status.Outgoing.Joined {
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
		For(&discoveryv1alpha1.ForeignCluster{}).
		Owns(&advtypes.Advertisement{}).
		Owns(&discoveryv1alpha1.PeeringRequest{}).
		Complete(r)
}

func (r *ForeignClusterReconciler) checkJoined(fc *discoveryv1alpha1.ForeignCluster, foreignDiscoveryClient *crdClient.CRDClient) (*discoveryv1alpha1.ForeignCluster, error) {
	_, err := foreignDiscoveryClient.Resource("peeringrequests").Get(fc.Status.Outgoing.RemotePeeringRequestName, metav1.GetOptions{})
	if err != nil {
		fc.Status.Outgoing.Joined = false
		fc.Status.Outgoing.RemotePeeringRequestName = ""
		tmp, err := r.crdClient.Resource("foreignclusters").Update(fc.Name, fc, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		var ok bool
		fc, ok = tmp.(*discoveryv1alpha1.ForeignCluster)
		if !ok {
			return nil, goerrors.New("updated object is not a ForeignCluster")
		}
	}
	return fc, nil
}

// this method returns the local setting (about trusting policy, ...) to be sent to the remote cluster
// to allow it to create its local ForeignCluster with the correct settings
func (r *ForeignClusterReconciler) getOriginClusterSets() discoveryv1alpha1.OriginClusterSets {
	if r.DiscoveryCtrl.Config == nil {
		klog.Warning("DiscoveryCtrl Config is not set, by default we are allowing UntrustedCA join")
		return discoveryv1alpha1.OriginClusterSets{
			AllowUntrustedCA: true,
		}
	} else {
		return discoveryv1alpha1.OriginClusterSets{
			AllowUntrustedCA: r.DiscoveryCtrl.Config.AllowUntrustedCA,
		}
	}
}

func (r *ForeignClusterReconciler) createPeeringRequestIfNotExists(clusterID string, owner *discoveryv1alpha1.ForeignCluster, foreignClient *crdClient.CRDClient) (*discoveryv1alpha1.PeeringRequest, error) {
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
			pr := &discoveryv1alpha1.PeeringRequest{
				TypeMeta: metav1.TypeMeta{
					Kind:       "PeeringRequest",
					APIVersion: "discovery.liqo.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: localClusterID,
				},
				Spec: discoveryv1alpha1.PeeringRequestSpec{
					ClusterID:         localClusterID,
					Namespace:         r.Namespace,
					KubeConfigRef:     nil,
					OriginClusterSets: r.getOriginClusterSets(),
				},
			}
			tmp, err = foreignClient.Resource("peeringrequests").Create(pr, metav1.CreateOptions{})
			if err != nil {
				return nil, err
			}
			var ok bool
			pr, ok = tmp.(*discoveryv1alpha1.PeeringRequest)
			if !ok {
				return nil, goerrors.New("created object is not a ForeignCluster")
			}
			secret := &apiv1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pr-" + clusterID,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "discovery.liqo.io/v1alpha1",
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
			pr.TypeMeta.APIVersion = "discovery.liqo.io/v1alpha1"
			tmp, err = foreignClient.Resource("peeringrequests").Update(pr.Name, pr, metav1.UpdateOptions{})
			if err != nil {
				return nil, err
			}
			pr, ok = tmp.(*discoveryv1alpha1.PeeringRequest)
			if !ok {
				return nil, goerrors.New("created object is not a ForeignCluster")
			}
			return pr, nil
		}
		// other errors
		return nil, err
	}
	// already exists
	pr, ok := tmp.(*discoveryv1alpha1.PeeringRequest)
	if !ok {
		return nil, goerrors.New("retrieved object is not a PeeringRequest")
	}
	return pr, nil
}

// this function return a kube-config file to send to foreign cluster and crate everything needed for it
func (r *ForeignClusterReconciler) getForeignConfig(clusterID string, owner *discoveryv1alpha1.ForeignCluster) (string, error) {
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

	// crdReplicator role binding
	err = r.setDispatcherRole(clusterID, sa)
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
		timeout := time.NewTimer(500 * time.Millisecond)
		ch := wa.ResultChan()
		defer timeout.Stop()
		defer wa.Stop()
		for iterate := true; iterate; {
			select {
			case s := <-ch:
				_sa := s.Object.(*apiv1.ServiceAccount)
				if _sa.Name == sa.Name && len(_sa.Secrets) > 0 {
					iterate = false
					break
				}
				break
			case <-timeout.C:
				// try to use default config
				if r.ForeignConfig != nil {
					klog.Warning("using default ForeignConfig")
					return r.ForeignConfig.String(), nil
				}
				// ServiceAccount not updated with secrets and no default config
				return "", errors.NewTimeoutError("ServiceAccount's Secret was not created", 0)
			}
		}
	}
	cnf, err := kubeconfig.CreateKubeConfig(r.crdClient.Client(), clusterID, r.Namespace)
	return cnf, err
}

func (r *ForeignClusterReconciler) createClusterRoleIfNotExists(clusterID string, owner *discoveryv1alpha1.ForeignCluster) (*rbacv1.ClusterRole, error) {
	role, err := r.crdClient.Client().RbacV1().ClusterRoles().Get(context.TODO(), clusterID, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// does not exist
		role = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1alpha1",
						Kind:       "ForeignCluster",
						Name:       owner.Name,
						UID:        owner.UID,
					},
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "create", "update", "delete", "watch"},
					APIGroups: []string{"sharing.liqo.io", ""},
					Resources: []string{"advertisements", "advertisements/status", "secrets"},
				},
			},
		}
		return r.crdClient.Client().RbacV1().ClusterRoles().Create(context.TODO(), role, metav1.CreateOptions{})
	} else if err != nil {
		klog.Error(err)
		return nil, err
	} else {
		return role, nil
	}
}

func (r *ForeignClusterReconciler) createServiceAccountIfNotExists(clusterID string, owner *discoveryv1alpha1.ForeignCluster) (*apiv1.ServiceAccount, error) {
	sa, err := r.crdClient.Client().CoreV1().ServiceAccounts(r.Namespace).Get(context.TODO(), clusterID, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// does not exist
		sa = &apiv1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1alpha1",
						Kind:       "ForeignCluster",
						Name:       owner.Name,
						UID:        owner.UID,
					},
				},
			},
		}
		return r.crdClient.Client().CoreV1().ServiceAccounts(r.Namespace).Create(context.TODO(), sa, metav1.CreateOptions{})
	} else if err != nil {
		klog.Error(err)
		return nil, err
	} else {
		return sa, nil
	}
}

func (r *ForeignClusterReconciler) createClusterRoleBindingIfNotExists(clusterID string, owner *discoveryv1alpha1.ForeignCluster) (*rbacv1.ClusterRoleBinding, error) {
	rb, err := r.crdClient.Client().RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterID, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// does not exist
		rb = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1alpha1",
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
	} else if err != nil {
		klog.Error(err)
		return nil, err
	} else {
		return rb, nil
	}
}

func (r *ForeignClusterReconciler) setDispatcherRole(clusterID string, sa *apiv1.ServiceAccount) error {
	_, err := r.crdClient.Client().RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterID+"-crdReplicator", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// does not exist
		rb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID + "-crdReplicator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "ServiceAccount",
						Name:       sa.Name,
						UID:        sa.UID,
					},
				},
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      sa.Name,
					Namespace: sa.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "crdReplicator-role",
			},
		}
		_, err = r.crdClient.Client().RbacV1().ClusterRoleBindings().Create(context.TODO(), rb, metav1.CreateOptions{})
		if err != nil {
			klog.Error(err)
		}
		return err
	} else if err != nil {
		klog.Error(err)
		return err
	} else {
		return nil
	}
}

func (r *ForeignClusterReconciler) deleteAdvertisement(fc *discoveryv1alpha1.ForeignCluster) error {
	return fc.DeleteAdvertisement(r.advertisementClient)
}

func (r *ForeignClusterReconciler) deletePeeringRequest(foreignClient *crdClient.CRDClient, fc *discoveryv1alpha1.ForeignCluster) error {
	return foreignClient.Resource("peeringrequests").Delete(fc.Status.Outgoing.RemotePeeringRequestName, metav1.DeleteOptions{})
}
