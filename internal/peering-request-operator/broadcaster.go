package peering_request_operator

import (
	"context"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"strings"
)

const (
	broadcasterCPU    = "100m"
	broadcasterMemory = "50M"
)

func (r *PeeringRequestReconciler) BroadcasterExists(request *discoveryv1alpha1.PeeringRequest) (bool, error) {
	_, err := r.crdClient.Client().AppsV1().Deployments(request.Status.BroadcasterRef.Namespace).Get(context.TODO(), request.Status.BroadcasterRef.Name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		// does not exist
		return false, nil
	}
	if err != nil {
		// other errors
		return false, err
	}
	// already exists
	return true, nil
}

func GetBroadcasterDeployment(request *discoveryv1alpha1.PeeringRequest, nameSA string, remoteSA string, namespace string, image string, clusterId string) *appsv1.Deployment {
	args := []string{
		"--peering-request",
		request.Name,
		"--cluster-id",
		clusterId,
		"--service-account",
		remoteSA,
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: strings.Join([]string{"broadcaster", request.Name, ""}, "-"),
			Namespace:    namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1alpha1",
					Kind:       "PeeringRequest",
					Name:       request.Name,
					UID:        request.UID,
					Controller: pointer.BoolPtr(true),
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "broadcaster-" + request.Name,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "broadcaster-" + request.Name,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "broadcaster-" + request.Name,
							Image:           image,
							ImagePullPolicy: v1.PullAlways,
							Args:            args,
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									"cpu":    resource.MustParse(broadcasterCPU),
									"memory": resource.MustParse(broadcasterMemory),
								},
								Requests: v1.ResourceList{
									"cpu":    resource.MustParse(broadcasterCPU),
									"memory": resource.MustParse(broadcasterMemory),
								},
							},
							Env: []v1.EnvVar{
								{
									Name:  "POD_NAMESPACE",
									Value: namespace,
								},
							},
						},
					},
					ServiceAccountName: nameSA,
				},
			},
		},
	}

	return deploy
}
