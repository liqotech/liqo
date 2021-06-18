package outgoing

import (
	"context"
	"testing"

	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog/v2"

	liqonetTest "github.com/liqotech/liqo/pkg/liqonet/test"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	api "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping/test"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
	storageTest "github.com/liqotech/liqo/pkg/virtualKubelet/storage/test"
)

var (
	localNode   = "worker-3"
	virtualNode = "vk-node"
)

func TestEndpointAdd(t *testing.T) {
	foreignClient := fake.NewSimpleClientset()
	cacheManager := &storageTest.MockManager{
		HomeCache:    map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
		ForeignCache: map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
	}
	nattingTable := &test.MockNamespaceMapper{Cache: map[string]string{}}

	Greflector := &api.GenericAPIReflector{
		ForeignClient:    foreignClient,
		NamespaceNatting: nattingTable,
		CacheManager:     cacheManager,
	}

	reflector := &EndpointSlicesReflector{
		APIReflector:    Greflector,
		VirtualNodeName: types.NewNetworkingOption("VirtualNodeName", "vk-node"),
		IpamClient:      &liqonetTest.MockIpam{LocalRemappedPodCIDR: "10.0.0.0/16"},
	}
	reflector.SetSpecializedPreProcessingHandlers()

	epslice := &discoveryv1.EndpointSlice{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "homeNamespace",
			Labels: map[string]string{
				"test": "true",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Service",
					Name:       "name",
					UID:        "f677f233-2cf8-4cae-8r5d-bbf3ea1d8671",
				},
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.15"},
				Conditions: discoveryv1.EndpointConditions{},
				Hostname:   &localNode,
				TargetRef:  nil,
			},
			{
				Addresses:  []string{"10.0.0.20"},
				Conditions: discoveryv1.EndpointConditions{},
				Hostname:   &virtualNode,
				TargetRef:  nil,
			}},
		Ports: nil,
	}

	svc := &v1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "homeNamespace-natted",
			Labels:    nil,
			UID:       "f677f0a3-2ce8-4cae-810d-bbf3ea1d8671",
		},
		Spec:   v1.ServiceSpec{},
		Status: v1.ServiceStatus{},
	}

	_, _ = nattingTable.NatNamespace("homeNamespace", true)
	_, err := reflector.GetForeignClient().CoreV1().Services("homeNamespace-natted").Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}

	pa, _ := reflector.PreProcessAdd(epslice)
	postadd := pa.(*discoveryv1.EndpointSlice)

	assert.Equal(t, postadd.Namespace, "homeNamespace-natted", "Asserting namespace natting")
	assert.Equal(t, len(postadd.Endpoints), 1, "Asserting node-based filtering")
	assert.Equal(t, postadd.Endpoints[0].Addresses[0], "10.0.0.15", "Asserting no pod IP natting")
}

func TestEndpointAdd2(t *testing.T) {
	foreignClient := fake.NewSimpleClientset()
	cacheManager := &storageTest.MockManager{
		HomeCache:    map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
		ForeignCache: map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
	}
	nattingTable := &test.MockNamespaceMapper{Cache: map[string]string{}}

	Greflector := &api.GenericAPIReflector{
		ForeignClient:    foreignClient,
		NamespaceNatting: nattingTable,
		CacheManager:     cacheManager,
	}

	reflector := &EndpointSlicesReflector{
		APIReflector:    Greflector,
		VirtualNodeName: types.NewNetworkingOption("VirtualNodeName", "vk-node"),
		IpamClient:      &liqonetTest.MockIpam{LocalRemappedPodCIDR: "10.0.0.0/16"},
	}
	reflector.SetSpecializedPreProcessingHandlers()

	epSlice := &discoveryv1.EndpointSlice{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "homeNamespace",
			Labels: map[string]string{
				"test": "true",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Service",
					Name:       "name",
					UID:        "f677f233-2cf8-4cae-8r5d-bbf3ea1d8671",
				},
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.10.0.15"},
				Conditions: discoveryv1.EndpointConditions{},
				Hostname:   &localNode,
				TargetRef:  nil,
			},
			{
				Addresses:  []string{"10.10.0.20"},
				Conditions: discoveryv1.EndpointConditions{},
				Hostname:   &virtualNode,
				TargetRef:  nil,
			}},
		Ports: nil,
	}

	svc := &v1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "homeNamespace-natted",
			Labels:    nil,
			UID:       "f677f0a3-2ce8-4cae-810d-bbf3ea1d8671",
		},
		Spec:   v1.ServiceSpec{},
		Status: v1.ServiceStatus{},
	}

	_, _ = nattingTable.NatNamespace("homeNamespace", true)
	_, err := reflector.GetForeignClient().CoreV1().Services("homeNamespace-natted").Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}

	pa, _ := reflector.PreProcessAdd(epSlice)
	postadd := pa.(*discoveryv1.EndpointSlice)

	assert.Equal(t, postadd.Namespace, "homeNamespace-natted", "Asserting namespace natting")
	assert.Equal(t, len(postadd.Endpoints), 1, "Asserting node-based filtering")
	assert.Equal(t, postadd.Endpoints[0].Addresses[0], "10.0.0.15", "Asserting pod IP natting")
}
