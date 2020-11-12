package reflection

import (
	"context"
	"github.com/liqotech/liqo/test/unit/virtualKubelet/utils"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/discovery/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"testing"
)

func TestEndpointAdd(t *testing.T) {
	epReflector := utils.InitTest("endpointSlices")
	if epReflector == nil {
		t.Fail()
	}

	epslice := v1beta1.EndpointSlice{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
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
		Endpoints: []v1beta1.Endpoint{
			{
				Addresses:  []string{"10.0.0.15"},
				Conditions: v1beta1.EndpointConditions{},
				Hostname:   nil,
				TargetRef:  nil,
				Topology:   map[string]string{"kubernetes.io/hostname": "worker-3"},
			},
			{
				Addresses:  []string{"10.0.0.20"},
				Conditions: v1beta1.EndpointConditions{},
				Hostname:   nil,
				TargetRef:  nil,
				Topology:   map[string]string{"kubernetes.io/hostname": "vk-node"},
			}},
		Ports: nil,
	}

	svc := v1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "test",
			Labels:    nil,
			UID:       "f677f0a3-2ce8-4cae-810d-bbf3ea1d8671",
		},
		Spec:   v1.ServiceSpec{},
		Status: v1.ServiceStatus{},
	}

	_, err := epReflector.GetForeignClient().CoreV1().Services("test").Create(context.TODO(), &svc, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}

	postadd := epReflector.PreProcessAdd(&epslice).(*v1beta1.EndpointSlice)

	assert.Equal(t, postadd.Namespace, "test", "Asserting namespace natting")
	assert.Equal(t, len(postadd.Endpoints), 1, "Asserting node-based filtering")
	assert.Equal(t, postadd.Endpoints[0].Addresses[0], "10.0.0.15", "Asserting no pod IP natting")
}

func TestEndpointAdd2(t *testing.T) {
	epReflector := utils.InitTest("endpointSlices")
	if epReflector == nil {
		t.Fail()
	}

	epslice := v1beta1.EndpointSlice{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
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
		Endpoints: []v1beta1.Endpoint{
			{
				Addresses:  []string{"10.10.0.15"},
				Conditions: v1beta1.EndpointConditions{},
				Hostname:   nil,
				TargetRef:  nil,
				Topology:   map[string]string{"kubernetes.io/hostname": "worker-3"},
			},
			{
				Addresses:  []string{"10.10.0.20"},
				Conditions: v1beta1.EndpointConditions{},
				Hostname:   nil,
				TargetRef:  nil,
				Topology:   map[string]string{"kubernetes.io/hostname": "vk-node"},
			}},
		Ports: nil,
	}

	svc := v1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "test",
			Labels:    nil,
			UID:       "f677f0a3-2ce8-4cae-810d-bbf3ea1d8671",
		},
		Spec:   v1.ServiceSpec{},
		Status: v1.ServiceStatus{},
	}

	_, err := epReflector.GetForeignClient().CoreV1().Services("test").Create(context.TODO(), &svc, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}

	postadd := epReflector.PreProcessAdd(&epslice).(*v1beta1.EndpointSlice)

	assert.Equal(t, postadd.Namespace, "test", "Asserting namespace natting")
	assert.Equal(t, len(postadd.Endpoints), 1, "Asserting node-based filtering")
	assert.Equal(t, postadd.Endpoints[0].Addresses[0], "10.0.0.15", "Asserting pod IP natting")
}
