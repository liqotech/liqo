package federation_request_operator

import (
	discoveryv1 "github.com/netgroup-polito/dronev2/api/discovery/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func GetBroadcasterDeployment(request *discoveryv1.FederationRequest, nameSA string, namespace string, image string) appsv1.Deployment {
	/*args := []string{
		"--federation-request",
		request.Name,
	}*/

	deploy := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "broadcaster-" + request.Name,
			Namespace: namespace,
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
							//Command:      command,
							//Args:         args,
						},
					},
					ServiceAccountName: nameSA,
				},
			},
		},
	}

	return deploy
}
