// Copyright 2019-2023 The Liqo Authors
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

package shadowpod

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	testutil "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var (
	scheme  *runtime.Scheme
	decoder *admission.Decoder
	ctx     = context.Background()

	tenantNamespace                   = "tenant-namespace"
	tenantNamespace2                  = "tenant-namespace-2"
	testNamespace                     = "test-namespace"
	testNamespace2                    = "test-namespace-2"
	testNamespaceInvalid              = "test-namespace-invalid"
	testShadowPodName                 = "test-shadowpod"
	testShadowPodName2                = "test-shadowpod-2"
	testShadowPodUID        types.UID = "test-shadowpod-uid"
	testShadowPodUID2       types.UID = "test-shadowpod-uid-2"
	testShadowPodUID3       types.UID = "test-shadowpod-uid-3"
	testShadowPodUID4       types.UID = "test-shadowpod-uid-4"
	testShadowPodUIDInvalid types.UID = "test-shadowpod-uid-invalid"
	clusterID                         = "test-cluster-id"
	clusterID2                        = "test-cluster-id-2"
	clusterID3                        = "test-cluster-id-3"
	clusterIDInvalid                  = "test-cluster-id-invalid"
	clusterName                       = "test-cluster"
	clusterName2                      = "test-cluster-2"
	clusterName3                      = "test-cluster-3"
	resourceCPU                       = 1000000
	resourceMemory                    = 1000000
	resourceQuota                     = forgeResourceList(int64(resourceCPU), int64(resourceMemory))
	resourceQuota2                    = forgeResourceList(int64(resourceCPU/2), int64(resourceMemory/2))
	resourceQuota4                    = forgeResourceList(int64(resourceCPU/4), int64(resourceMemory/4))
	clusterIdentity                   = forgeClusterIdentity(clusterName, clusterID)
	clusterIdentity2                  = forgeClusterIdentity(clusterName2, clusterID2)
	foreignCluster                    = forgeForeignCluster(clusterName, clusterID)
	foreignCluster2                   = forgeForeignCluster(clusterName2, clusterID2)
	resourceOffer                     = forgeResourceOfferWithLabel(clusterName, tenantNamespace, clusterID)
	resourceOffer2                    = forgeResourceOfferWithLabel(clusterName2, tenantNamespace2, clusterID2)
	fakeShadowPod                     = forgeShadowPod(nsName.Name, nsName.Namespace, string(testShadowPodUID), clusterID)
	fakeShadowPod2                    = forgeShadowPod(nsName2.Name, nsName2.Namespace, string(testShadowPodUID2), clusterID)
	nsName                            = types.NamespacedName{Name: testShadowPodName, Namespace: testNamespace}
	nsName2                           = types.NamespacedName{Name: testShadowPodName2, Namespace: testNamespace}
	nsName3                           = types.NamespacedName{Name: testShadowPodName + "-3", Namespace: testNamespace2}
	nsName4                           = types.NamespacedName{Name: testShadowPodName + "-4", Namespace: testNamespace2}
	freeQuotaZero                     = &corev1.ResourceList{
		corev1.ResourceCPU:    *resource.NewQuantity(0, resource.DecimalSI),
		corev1.ResourceMemory: *resource.NewQuantity(0, resource.DecimalSI),
	}
)

type containerResource struct {
	cpu    int64
	memory int64
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	testutil.LogsToGinkgoWriter()
	Expect(vkv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(sharingv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(discoveryv1alpha1.AddToScheme(scheme)).To(Succeed())
	var err error
	decoder, err = admission.NewDecoder(scheme)
	Expect(err).ToNot(HaveOccurred())
})

func TestShadowpod(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shadowpod Suite")
}

func serializeShadowPod(sp *vkv1alpha1.ShadowPod) runtime.RawExtension {
	data, err := json.Marshal(sp)
	Expect(err).ToNot(HaveOccurred())
	return runtime.RawExtension{Raw: data}
}

func forgeNamespaceWithClusterID(clusterID string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
			Labels: map[string]string{
				consts.RemoteClusterID: clusterID,
			},
		},
	}
}

func forgeRequest(op admissionv1.Operation, newShadowPod, oldShadowPod *vkv1alpha1.ShadowPod) admission.Request {
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Operation: op}}
	if oldShadowPod != nil {
		req.OldObject = serializeShadowPod(oldShadowPod)
		req.Name = oldShadowPod.Name
	}
	if newShadowPod != nil {
		req.Object = serializeShadowPod(newShadowPod)
		req.Name = newShadowPod.Name
	}
	req.DryRun = pointer.Bool(false)
	return req
}

func forgeResourceList(cpu, memory int64, gpu ...int64) *corev1.ResourceList {
	resourceList := corev1.ResourceList{}
	if cpu > 0 {
		resourceList[corev1.ResourceCPU] = *resource.NewQuantity(cpu, resource.DecimalSI)
	}
	if memory > 0 {
		resourceList[corev1.ResourceMemory] = *resource.NewQuantity(memory, resource.DecimalSI)
	}
	if len(gpu) > 0 {
		resourceList[corev1.ResourceName("nvidia.com/gpu")] = *resource.NewQuantity(gpu[0], resource.DecimalSI)
	}
	return &resourceList
}

func forgeShadowPodWithClusterID(clusterID, namespace string) *vkv1alpha1.ShadowPod {
	return &vkv1alpha1.ShadowPod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testShadowPodName,
			Namespace: namespace,
			Labels: map[string]string{
				forge.LiqoOriginClusterIDKey: clusterID,
			},
		},
	}
}

func forgeShadowPod(name, namespace, uid, clusterID string) *vkv1alpha1.ShadowPod {
	return &vkv1alpha1.ShadowPod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uid),
			Labels: map[string]string{
				forge.LiqoOriginClusterIDKey: clusterID,
			},
		},
		Spec: vkv1alpha1.ShadowPodSpec{
			Pod: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-container",
						Image: "test-image",
						Resources: corev1.ResourceRequirements{
							Limits: *forgeResourceList(int64(resourceCPU/4), int64(resourceMemory/4)),
						},
					},
				},
			},
		},
	}
}

func forgeShadowPodWithResourceLimits(containers, initContainer []containerResource) *vkv1alpha1.ShadowPod {
	sp := &vkv1alpha1.ShadowPod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testShadowPodName,
			Namespace: testNamespace,
			UID:       testShadowPodUID,
			Labels: map[string]string{
				forge.LiqoOriginClusterIDKey: clusterID,
			},
		},
	}
	if len(containers) > 0 {
		sp.Spec.Pod.Containers = make([]corev1.Container, len(containers))
		for i, container := range containers {
			sp.Spec.Pod.Containers[i] = corev1.Container{
				Name:  "test-container",
				Image: "test-image",
				Resources: corev1.ResourceRequirements{
					Limits: *forgeResourceList(container.cpu, container.memory),
				},
			}
		}
	}
	if len(initContainer) > 0 {
		sp.Spec.Pod.InitContainers = make([]corev1.Container, len(initContainer))
		for i, container := range initContainer {
			sp.Spec.Pod.InitContainers[i] = corev1.Container{
				Name:  "test-init-container",
				Image: "test-image",
				Resources: corev1.ResourceRequirements{
					Limits: *forgeResourceList(container.cpu, container.memory),
				},
			}
		}
	}
	return sp
}

func forgeShadowPodList(shadowPods ...*vkv1alpha1.ShadowPod) *vkv1alpha1.ShadowPodList {
	spList := &vkv1alpha1.ShadowPodList{}

	for _, sp := range shadowPods {
		spList.Items = append(spList.Items, *sp)
	}

	return spList
}

func forgeResourceOfferWithLabel(clustername, namespace, clusterID string) *sharingv1alpha1.ResourceOffer {
	ro := &sharingv1alpha1.ResourceOffer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clustername,
			Namespace: namespace,
		},
		Spec: sharingv1alpha1.ResourceOfferSpec{
			ResourceQuota: corev1.ResourceQuotaSpec{
				Hard: *forgeResourceList(int64(resourceCPU), int64(resourceMemory)),
			},
		},
	}
	if clusterID != "" {
		ro.Labels = map[string]string{
			discovery.ClusterIDLabel:           clusterID,
			consts.ReplicationDestinationLabel: clusterID,
			consts.ReplicationRequestedLabel:   "true",
		}
	}
	return ro
}

func forgeForeignCluster(clustername, clusterID string) *discoveryv1alpha1.ForeignCluster {
	return &discoveryv1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: clustername,
			Labels: map[string]string{
				discovery.ClusterIDLabel: clusterID,
			},
		},
		Spec: discoveryv1alpha1.ForeignClusterSpec{
			ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
				ClusterID:   clusterID,
				ClusterName: clustername,
			},
		},
	}
}

func forgeClusterIdentity(clustername, clusterid string) *discoveryv1alpha1.ClusterIdentity {
	return &discoveryv1alpha1.ClusterIdentity{ClusterID: clusterid,
		ClusterName: clustername,
	}
}
