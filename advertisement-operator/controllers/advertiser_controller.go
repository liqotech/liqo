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

package controllers

import (
	"context"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	advertisementv1beta1 "github.com/netgroup-polito/dronev2/advertisement-operator/api/v1beta1"
)

// AdvertiserReconciler reconciles a Advertiser object
type AdvertiserReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=advertisement.drone.com,resources=advertisers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=advertisement.drone.com,resources=advertisers/status,verbs=get;update;patch

func (r *AdvertiserReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("advertisement", req.NamespacedName)

	// your logic here

	providerConfigMap := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "vk-config",
			Namespace: 					"default",
		},
		Data:       map[string]string{
				"vkubelet-cfg.json" : `
    {
      "virtual-kubelet": {
        "remoteKubeconfig" : "/app/kubeconfig/remote",
        "namespace": "drone-v2",
        "cpu": "2",
        "memory": "32Gi",
        "pods": "128"
      }
    }`},
	}

	if err := r.Create(ctx, &providerConfigMap, &client.CreateOptions{}) ; err != nil && !errors.IsAlreadyExists(err){
		log.Error(err, "unable to create configMap")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	remoteKubeConfigMap := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "remote-kubeconfig",
			Namespace: "default",
		},
		Data:       map[string]string{
			"remote" : `
    apiVersion: v1
    clusters:
    - cluster:
        certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN5RENDQWJDZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREFWTVJNd0VRWURWUVFERXdwcmRXSmwKY201bGRHVnpNQjRYRFRJd01ERXhNREV6TXpreU1sb1hEVE13TURFd056RXpNemt5TWxvd0ZURVRNQkVHQTFVRQpBeE1LYTNWaVpYSnVaWFJsY3pDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUJBTUNlCkJGNkx3UFNGNmRRam5WTzR4UC9qeTVOcDMzemhISkdUb1dWdnYvZGp0OFBFbEIzU1lCSmJDY3VtMnMxNEUzLysKRzdwc1p6VHU0cXhWNUhuYXFwK0tLSWRlN3B1ZGtEc2RqZjVGQUZVUHRFVllDL2JVS21OSWhSL0dSK2JMNEFqVQpsT1o0TlhONmJVcTJ3V3dKcUQ2R25jRjBlcWdPdXB2YzZJajlhYmJ2eHA4bjVjM3FqV1d1YnFVSTJWOFNwK0dNCm9lUm01bFdUZ1JpTTRWNTZpWUZTcld3bHQ2d3dsZUthdmM1SjZzVTVSTzJyNHdXeDdXc29McDlmWkpISWRzS3UKSnQrUHk5TGZQQ05jekgzY3IwaXBZOWdZU1JJTFJTMFducVE5SS8vRWRTQk9XU3lRbXphM0NLUFFMcFM3eHBWRwpOWVRWSHNzNXVJd0QrOE1hN3hNQ0F3RUFBYU1qTUNFd0RnWURWUjBQQVFIL0JBUURBZ0trTUE4R0ExVWRFd0VCCi93UUZNQU1CQWY4d0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dFQkFIVWJvYjBKUzdldWN2RDNUcklZWG9OWGQ3ZlEKRmppRWZIcVd2QjZCWkdLS1JKWHJab3RHTDF3c3dmNFQ3TXdLakZQUkJBdjNlQnJXaWwxRGhCR2hsU2J1TTVXTApuZExVRW5LalNoUzByY3hTUkxmQ0d5bWllckpWaVFMWC9FNzhZYUF5OVRWQ1Qyc2ZTa2JhR3p6eUN4ZkdDQWdhCktsWjlJRys5MXFLd2JkR0dtSUdaNFNNZnZDNUw4a2owOFQxSzZOT0lSNUpIbGFZaUZZQUEwY3paaHRETEV5OGEKWWhPdk1kV1ZxRFdvdXJ5cE5sOGxteXJuQURERlUzL0l2aUFCajBVM2psZFI0WjVGNDlFbW5icW8zMy9ZQ0ZaaQpTeFRXSllNT1dCV3EyVGw3Rytpc1dlSGdIeHdBZ1VyanFGaEdYMTVIMDZLM09ucFVNMk1vR0lXN1dybz0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
        server: https://127.0.0.1:38103
        name: kind-remote
    contexts:
    - context:
        cluster: kind-remote
        user: kind-remote
      name: kind-remote
    current-context: kind-remote
    kind: Config
    preferences: {}
    users:
    - name: kind-remote
      user:
        client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUM4akNDQWRxZ0F3SUJBZ0lJZkg2WGs4MndTWWd3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TURBeE1UQXhNek01TWpKYUZ3MHlNVEF4TURreE16TTVNalZhTURReApGekFWQmdOVkJBb1REbk41YzNSbGJUcHRZWE4wWlhKek1Sa3dGd1lEVlFRREV4QnJkV0psY201bGRHVnpMV0ZrCmJXbHVNSUlCSWpBTkJna3Foa2lHOXcwQkFRRUZBQU9DQVE4QU1JSUJDZ0tDQVFFQXhSOTBLcy9pOHhRL3hobmoKMXpuWEFORzI5K1VjZ0FST2s4b1ljN3BBT3lMOTFsd0pxVHgycVczNVZ0YXU0VFdOSkVsVWppRFpGVk5zVWQ5Swo4N0E4ckxETG9wVEhEbEFtZzZ1d2ppczZscEZvZTMwTytTY1dTNzlld2tHTE9UdERwelBValVMVFB4WkFKK0QwCis2dmJjTncrOEZSZnhlelFEUDNrd1NxSWp0enZ3K2hqOWovaUh3aHc4cEo5Yk5iYVU4NGpERVFQMTQxS3VGUC8KandnKzNKdFl6blNkNlB1OUwzbFU0V3Rmcjhsa0djaDRtZUhjd2x5RU5uT3BCQmhPV2xLTE5SUjJEOEs5aldmUgpLTW01a0czOUdLTEE5WHFhQjIxbkxMbzRmRGVHSmRmVmtoOHdxK0FBV1RYUENMb1BoU2NEd2hFYnpWeC91QkZqCkpKcTQ2UUlEQVFBQm95Y3dKVEFPQmdOVkhROEJBZjhFQkFNQ0JhQXdFd1lEVlIwbEJBd3dDZ1lJS3dZQkJRVUgKQXdJd0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dFQkFHMG5Dc1FLcHJsNUpkeWhTOFBTSmJCOFhNQ2NZWldjc2FJUQpUZ3lNU1JQMG5QMVVBSnNwWCtjc3FpQlMvZDZhSkQ1c01iL0ZZSUovdjQ3dHJZUTJVT1BOWEZZYzdSbE83WUFSClFUbjRyVHlwK2FhdTZVbnVWYjNjWDVCZEhqUUl3UVFaWmc2WjFEODJIbzk2SDFNTHIySldjdHlZd0lXdkE3K0MKaW1RbzVsZldtTHdLK3dLNjlCZE5oTEpYYlhTZXRXWklQZzMwaTVZa0k1Y2lzM1pWYlBWTGlkUGJCcTRjS3ZaMwpvQWM4c2txVXpsYW1nWW1TM2ExYVNzakNhUk9oakUzUk51MTBJeU9YWm9RaVhtZndYVDdsWXE4WllCaWZEUGdJCjBwdm9kRTFtaHQ3RlNiL25rVHQ2bE56d1d1SkZ4OUZEMnVRQnVlMlRZYk1hL2xJbElKbz0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
        client-key-data: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcFFJQkFBS0NBUUVBeFI5MEtzL2k4eFEveGhuajF6blhBTkcyOStVY2dBUk9rOG9ZYzdwQU95TDkxbHdKCnFUeDJxVzM1VnRhdTRUV05KRWxVamlEWkZWTnNVZDlLODdBOHJMRExvcFRIRGxBbWc2dXdqaXM2bHBGb2UzME8KK1NjV1M3OWV3a0dMT1R0RHB6UFVqVUxUUHhaQUorRDArNnZiY053KzhGUmZ4ZXpRRFAza3dTcUlqdHp2dytoago5ai9pSHdodzhwSjliTmJhVTg0akRFUVAxNDFLdUZQL2p3ZyszSnRZem5TZDZQdTlMM2xVNFd0ZnI4bGtHY2g0Cm1lSGN3bHlFTm5PcEJCaE9XbEtMTlJSMkQ4SzlqV2ZSS01tNWtHMzlHS0xBOVhxYUIyMW5MTG80ZkRlR0pkZlYKa2g4d3ErQUFXVFhQQ0xvUGhTY0R3aEVielZ4L3VCRmpKSnE0NlFJREFRQUJBb0lCQVFDaHBoU09VeUlLTW0zRgoxMDlYSE5CcWxJK1ZvK1dHT1lIeWdnVWhTZmdJUnI2Z1BhT1NpOG5IVVM3cWZteTB0RVNGSURsZHdDUWs3OTloCkdGcVBrZE4vemc5L3lMK2Z4aXgvUjVrbVRON2p3R1JNK0JZQ1RZSmtKWS9nZnYxYnRuVFpOWkMxTXJxbS9ta28KQ3JyN2MwZ2x1Z2RHNy9vR0JoZnF2MzRLeGdWc0dPTHlHcEhnam1OQm1OeU1Zd2F5dHR6V1N4WjFQS2t4bDd6Mgo1YkV4TjljdjBSRXgvbndOSUZXQnZ3UGpMUU9kNEt4aG9URzR2QWpaK0o4UG1RdTE0Wk5GTTBRMTdCK2d1RVFqClB0a0h3bWgwNlhsS3RxWUZnUlIxQU0wbVdKRmhDbU9QVVZRSkFNb0F5QUkyWEhVWHJ3Umh4V2tCRUQ1bW9aaEwKUUNycTVTY0JBb0dCQU8wT21RVE1FUHVsN0l5VGl0blBMNE9EYi9SZThVbkNmYkFzUzRiTkJkWTB4QmJGYjdNcgpOaXdVTW9HSnBUS3JLNlA5bHJxQTNKb1pGYXZzb2ZKbWp2V282VW9HRmNFc3U1ZmUxY201QUhTTSs3T0J4aHlpCjRzTGoxM1ZFc1ZRc1ZFbjMrWFBtUGNtV1FBVDN2UnBNYXZWSWhFWFBtZWZYWDhFYU1HOUw4TUloQW9HQkFOVGYKNzNOMkNOVDl6cmduOHFIUHpoRmFad0tYRXFmOFgxMUc2SG0zT1pKckI2bm1yNkgwRmt0SjhqSWVtcWlIYnJFRApjTjJaT0NvSGxKdC9KU05rbWxVQjEvTVhZRmxncit6UVdrUnFPdUE3Z2VIRDNldDBOQW1mQ2E1S0FCbVMyUmRHCkRzekhwdW1vQ0hlb0NKSzVZanlldzFIc3pQNmJSS1RWUm1YcElxM0pBb0dBSzFHbmxNRFZ1YWF3ZTEvYTE4S00KcERPNG1hZGY0R0t5SlNkekJjY2hjZXRpaWVhNmFydFN3dXRONzIzL3lpcU5ad0pJTVB5clUxMlNJRUMxdDE0VwpjYjNVSTdySTd1d0Z1OUwwcmxBb1RTUVdPczlVTEpkM2FMWEtBWnZ6NjdYT0VWWkhOMjZ6aThyeEYvZE5qeWkzCnd1cmxnUHhXMjQ3MzZJbW9vQzM0YVFFQ2dZRUF6N2toUU1ycVBXVFo1bXZjNExjVnYyczIzNWtwdEZDWmdqemkKTjN0cXE0elRjcUJQdkRxaDBwLzZ2WnVOa1d4dXdENjZVUkxsY21YcFJuOGdiMVFKSVhCbUdLa3o5S05icUR0OApDZ3liSFJvVVdJaTNzYjIzMWJlaVM3ZWNOMWhMak9GcEtieWRESjVTZk9pMFRQQ25ncjN0bkxEMUxIRzQzeHZhCjBURlpETGtDZ1lFQW0rOGJtdWlETlRKN09kSnc2RHJEVG45OEJNaHU5Y1drRTJYTXpBVlRIWEx5VDBhT3ArdSsKbmoyMFpzZDlzbExQNnBGa3Z1d2djc05uVkRwS1pZZWpscmtVVko4dWh5Y2RKcUV4Y1FMOVFtS05CdWk1dEc1NgprWFE4dnBvNi9mRXVTbzhya1VvTXhwQzVMVTFRWGgzV0FmNEZqR3lFb001QnR1ekZvaVlWdkVJPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=`},
	}

	if err := r.Create(ctx, &remoteKubeConfigMap, &client.CreateOptions{}) ; err != nil && !errors.IsAlreadyExists(err){
		log.Error(err, "unable to create configMap")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	serviceAccount := v1.ServiceAccount{
		ObjectMeta:                   metav1.ObjectMeta{
			Name:                       "virtual-kubelet",
			Namespace:					"default",
		},
	}

	if err := r.Create(ctx, &serviceAccount, &client.CreateOptions{}) ; err != nil && !errors.IsAlreadyExists(err){
		log.Error(err, "unable to create serviceAccount")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	subject := make([]rbacv1.Subject, 1)
	subject[0] = rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      "virtual-kubelet",
		Namespace: "default",
	}

	clusterRoleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "virtual-kubelet",
		},
		Subjects:   subject,
		RoleRef:    rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	}

	if err := r.Create(ctx, &clusterRoleBinding, &client.CreateOptions{}) ; err != nil && !errors.IsAlreadyExists(err){
		log.Error(err, "unable to create clusterRoleBinding")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	command := make([]string, 1)
	command[0] = "/usr/bin/virtual-kubelet"
	args := make([]string, 5)
	args[0] = "--provider" ; args[1] = "kubernetes"
	args[2] = "--provider-config" ; args[3] = "/app/config/vkubelet-cfg.json"
	args[4] = "--disable-taint"

	volumes := make([]v1.Volume, 2)

	volumes[0] = v1.Volume{
		Name:         "provider-config",
		VolumeSource: v1.VolumeSource{
			ConfigMap:            &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{Name : "vk-config"},
			},
		},
	}
	volumes[1] = v1.Volume{
		Name:         "remote-kubeconfig",
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{Name : "remote-kubeconfig"},
			},
		},
	}

	volumeMounts := make([]v1.VolumeMount, 2)

	volumeMounts[0] = v1.VolumeMount{
		Name:             "provider-config",
		MountPath:        "/app/config/vkubelet-cfg.json",
		SubPath:          "vkubelet-cfg.json",
	}

	volumeMounts[1] = v1.VolumeMount{
		Name:             "remote-kubeconfig",
		MountPath:        "/app/kubeconfig/remote",
		SubPath:          "remote",
	}

	deploy := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "virtual-kubelet",
			Namespace:                  "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "virtual-kubelet",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "virtual-kubelet",
					},
				},
				Spec: v1.PodSpec{
					Volumes:        volumes,
					Containers: []v1.Container{
						{
							Name:                     "virtual-kubelet",
							Image:                    "dronev2/virtual-kubelet",
							Command:                  command,
							Args:                     args,
							VolumeMounts:             volumeMounts,
						},
					},
					ServiceAccountName:            "virtual-kubelet",
				},
			},
		},
	}

	if err := r.Create(ctx, &deploy, &client.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err){
		log.Error(err, "unable to create virtual kubelet")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	/*
	var adv advertisementv1beta1.Advertiser
		if err := r.Get(ctx, req.NamespacedName, &adv); err != nil {
			log.Error(err, "unable to fetch Advertisement")
			// we'll ignore not-found errors, since they can't be fixed by an immediate
			// requeue (we'll need to wait for a new notification), and we can get them
			// on deleted requests.
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		resourceList := make(map[v1.ResourceName]resource.Quantity)
		resourceList["cpu"] = adv.Spec.Availability.Cpu
		resourceList["memory"] = adv.Spec.Availability.Ram


		images := make([]v1.ContainerImage, len(adv.Spec.Resources))
	for i := 0; i < len(adv.Spec.Resources); i++ {
		images[i] = adv.Spec.Resources[i].Image
	}

	node := v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "virtual-kubelet-1",
		},
		Spec: v1.NodeSpec{},
		Status: v1.NodeStatus{
			Capacity:    resourceList,
			Allocatable: resourceList,
			Images:      images,
		},
	}

	err := r.Create(ctx, &node, &client.CreateOptions{})
	if err != nil{
		log.Error(err, "unable to create Node")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}*/

	return ctrl.Result{}, nil
}

func (r *AdvertiserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&advertisementv1beta1.Advertiser{}).
		Complete(r)
}
