// Copyright 2019-2025 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package forge_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/pointer"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// FakeNodeLister implements the NodeLister interface.
var _ listerscorev1.NodeLister = &FakeNodeLister{}

type FakeNodeLister struct{}

// List lists all Nodes in the indexer.
func (fnl *FakeNodeLister) List(_ labels.Selector) (ret []*corev1.Node, err error) {
	return []*corev1.Node{}, nil
}

// Get retrieves the Node from the index for a given name.
func (fnl *FakeNodeLister) Get(name string) (*corev1.Node, error) {
	n := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{},
		}}
	if name == LiqoNodeName {
		n.Labels[consts.RemoteClusterID] = string(RemoteClusterID)
	}
	return n, nil
}

var _ = Describe("EndpointSlices Forging", func() {
	Translator := func(inputs []string) (outputs []string) {
		for _, input := range inputs {
			outputs = append(outputs, input+"-reflected")
		}
		return outputs
	}

	Describe("the RemoteEndpointSlice function", func() {
		var (
			input       *discoveryv1.EndpointSlice
			output      *offloadingv1beta1.ShadowEndpointSlice
			forgingOpts *forge.ForgingOpts
		)

		BeforeEach(func() {
			input = &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name", Namespace: "original",
					Labels:      map[string]string{"foo": "bar", testutil.FakeNotReflectedLabelKey: "true"},
					Annotations: map[string]string{"bar": "baz", testutil.FakeNotReflectedAnnotKey: "true"},
				},
				AddressType: discoveryv1.AddressTypeFQDN,
				Endpoints:   []discoveryv1.Endpoint{{Hostname: pointer.String("Test")}},
				Ports:       []discoveryv1.EndpointPort{{Name: pointer.String("HTTPS")}},
			}

			forgingOpts = testutil.FakeForgingOpts()

			JustBeforeEach(func() {
				output = forge.RemoteShadowEndpointSlice(input, output, &FakeNodeLister{}, "reflected", Translator, forgingOpts)
			})

			It("should correctly set the name and namespace", func() {
				Expect(output.Name).To(PointTo(Equal("name")))
				Expect(output.Namespace).To(PointTo(Equal("reflected")))
			})

			It("should correctly set the labels", func() {
				Expect(output.Labels).To(HaveKeyWithValue("foo", "bar"))
				Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
				Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
				Expect(output.Labels).To(HaveKeyWithValue(discoveryv1.LabelManagedBy, forge.EndpointSliceManagedBy))
				Expect(output.Labels).ToNot(HaveKey(testutil.FakeNotReflectedLabelKey))
			})
			It("should correctly set the annotations", func() {
				Expect(output.Annotations).To(HaveKeyWithValue("bar", "baz"))
				Expect(output.Annotations).ToNot(HaveKey(testutil.FakeNotReflectedAnnotKey))
			})
			It("should correctly set the address type", func() {
				Expect(output.Spec.Template.AddressType).To(PointTo(Equal(discoveryv1.AddressTypeFQDN)))
			})
			It("should correctly translate the endpoints", func() {
				Expect(output.Spec.Template.Endpoints).To(HaveLen(1))
				Expect(output.Spec.Template.Endpoints[0].Hostname).To(PointTo(Equal("Test")))
			})
			It("should correctly translate the ports", func() {
				Expect(output.Spec.Template.Ports).To(HaveLen(1))
				Expect(output.Spec.Template.Ports[0].Name).To(PointTo(Equal("HTTPS")))
			})
		})
	})

	Describe("the RemoteEndpointSliceEndpoints function", func() {
		var (
			endpoint discoveryv1.Endpoint
			input    []discoveryv1.Endpoint
			output   []discoveryv1.Endpoint
		)

		BeforeEach(func() {
			endpoint = discoveryv1.Endpoint{
				Addresses: []string{"first", "second"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: pointer.Bool(true), Serving: pointer.Bool(true), Terminating: pointer.Bool(true),
				},
				Hostname:  pointer.String("foo.bar.com"),
				NodeName:  pointer.String("whatever"),
				Zone:      pointer.String("target-zone"),
				Hints:     &discoveryv1.EndpointHints{ForZones: []discoveryv1.ForZone{{Name: "zone"}}},
				TargetRef: &corev1.ObjectReference{Kind: "Pod"},
			}
		})

		JustBeforeEach(func() {
			output = forge.RemoteEndpointSliceEndpoints(input, &FakeNodeLister{}, Translator)
		})

		When("translating a single endpoint", func() {
			BeforeEach(func() { input = []discoveryv1.Endpoint{endpoint} })
			It("should return a single endpoint", func() { Expect(output).To(HaveLen(1)) })
			It("should correctly translate and replicate the addresses", func() {
				Expect(output[0].Addresses).To(ConsistOf("first-reflected", "second-reflected"))
			})
			It("should correctly replicate the conditions", func() {
				Expect(output[0].Conditions).ToNot(BeNil())
				Expect(output[0].Conditions.Ready).To(PointTo(BeTrue()))
				// These are currently guarded by a feature gate, hence they are not reflected.
				Expect(output[0].Conditions.Serving).To(BeNil())
				Expect(output[0].Conditions.Terminating).To(BeNil())
			})
			It("should correctly translate and replicate the topology information", func() {
				Expect(output[0].NodeName).To(PointTo(Equal(string(LocalClusterID))))
				Expect(output[0].Zone).To(PointTo(Equal("target-zone")))
			})
			It("should correctly replicate the secondary fields", func() {
				Expect(output[0].Hostname).To(PointTo(Equal("foo.bar.com")))
				Expect(output[0].TargetRef).ToNot(BeNil())
				Expect(output[0].TargetRef.Kind).To(Equal("RemotePod"))
				Expect(output[0].Hints).ToNot(BeNil())
				Expect(output[0].Hints.ForZones).To(HaveLen(1))
				Expect(output[0].Hints.ForZones[0].Name).To(Equal("zone"))
			})
		})

		When("translating an endpoint referring to the remote cluster", func() {
			BeforeEach(func() {
				endpoint.NodeName = pointer.String(forge.LiqoNodeName)
				input = []discoveryv1.Endpoint{endpoint, endpoint, endpoint}
			})
			It("should return no endpoints", func() { Expect(output).To(HaveLen(0)) })
		})

		When("translating an endpoint with empty NodeName (e.g., external endpoint)", func() {
			BeforeEach(func() {
				endpoint.NodeName = nil
				input = []discoveryv1.Endpoint{endpoint}
			})
			It("should return the translated endpoint", func() { Expect(output).To(HaveLen(1)) })
		})

		When("translating multiple endpoints", func() {
			BeforeEach(func() { input = []discoveryv1.Endpoint{endpoint, endpoint, endpoint} })
			It("should return the correct number of endpoints", func() { Expect(output).To(HaveLen(3)) })
		})
	})

	Describe("the RemoteEndpointSlicePorts function", func() {
		var (
			input  discoveryv1.EndpointPort
			output []discoveryv1.EndpointPort
		)

		BeforeEach(func() { input = discoveryv1.EndpointPort{} })
		JustBeforeEach(func() { output = forge.RemoteEndpointSlicePorts([]discoveryv1.EndpointPort{input, input}) })

		When("the ports are correctly initialized", func() {
			BeforeEach(func() {
				input.Name = pointer.String("HTTPS")
				input.Port = pointer.Int32(443)
				input.Protocol = (*corev1.Protocol)(pointer.String(string(corev1.ProtocolTCP)))
				input.AppProtocol = pointer.String("protocol")
			})

			It("should return the correct number of ports", func() { Expect(output).To(HaveLen(2)) })
			It("should correctly replicate the port fields", func() {
				Expect(output[0].Name).To(PointTo(Equal("HTTPS")))
				Expect(output[0].Port).To(PointTo(BeNumerically("==", 443)))
				Expect(output[0].Protocol).To(PointTo(Equal(corev1.ProtocolTCP)))
				Expect(output[0].AppProtocol).To(PointTo(Equal("protocol")))
			})
		})

		When("the ports are not initialized", func() {
			It("should return the correct number of ports", func() { Expect(output).To(HaveLen(2)) })
			It("should leave all port fields nil", func() {
				Expect(output[0].Name).To(BeNil())
				Expect(output[0].Port).To(BeNil())
				Expect(output[0].Protocol).To(BeNil())
				Expect(output[0].AppProtocol).To(BeNil())
			})
		})
	})
})
