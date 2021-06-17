package natmappinginflater

import (
	"crypto/rand"
	"fmt"
	"math/big"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	liqonetapi "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// ForgeNatMapping forges a NatMapping resource for a cluster received as parameter.
func ForgeNatMapping(clusterID, podCIDR, externalCIDR string, mappings map[string]string) (*unstructured.Unstructured, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return nil, err
	}
	natMapping := &liqonetapi.NatMapping{
		TypeMeta: v1.TypeMeta{
			APIVersion: "net.liqo.io/v1alpha1",
			Kind:       "NatMapping",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: fmt.Sprintf("natmapping-%d", n),
			Labels: map[string]string{
				"net.liqo.io/natmapping":     "true",
				liqoconst.ClusterIDLabelName: clusterID,
			},
		},
		Spec: liqonetapi.NatMappingSpec{
			ClusterID:       clusterID,
			PodCIDR:         podCIDR,
			ExternalCIDR:    externalCIDR,
			ClusterMappings: mappings,
		},
	}

	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(natMapping)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: unstructuredObj}, nil
}
