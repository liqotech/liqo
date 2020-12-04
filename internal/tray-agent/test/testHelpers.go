package test

import (
	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreatePeeringRequest(clusterID string, clusterName string) *v1alpha1.PeeringRequest {
	return &v1alpha1.PeeringRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterID,
		},
		Spec: v1alpha1.PeeringRequestSpec{
			ClusterIdentity: v1alpha1.ClusterIdentity{
				ClusterID:   clusterID,
				ClusterName: clusterName,
			},
		},
	}
}

func CreateForeignCluster(clusterID string, clusterName string) *v1alpha1.ForeignCluster {
	pr := CreatePeeringRequest(clusterID, clusterName)

	fc := &v1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: pr.Spec.ClusterIdentity.ClusterID,
		},
		Spec: v1alpha1.ForeignClusterSpec{
			ClusterIdentity: pr.Spec.ClusterIdentity,
			Namespace:       pr.Spec.Namespace,
			Join:            false,
		},
	}
	return fc
}
