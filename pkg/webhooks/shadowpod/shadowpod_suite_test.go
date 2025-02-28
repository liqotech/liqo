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

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	testutil "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var (
	scheme *runtime.Scheme
	ctx    = context.Background()

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
	clusterID                         = liqov1beta1.ClusterID("test-cluster-id")
	clusterID2                        = liqov1beta1.ClusterID("test-cluster-id-2")
	clusterIDInvalid                  = liqov1beta1.ClusterID("test-cluster-id-invalid")
	userName                string    = "test-user-name"
	userName2               string    = "test-user-name-2"
	userName3               string    = "test-user-name-3"
	userNameInvalid         string    = "test-user-name-invalid"
	resourceCPU                       = 1000000
	resourceMemory                    = 1000000
	resourceQuota                     = forgeResourceList(int64(resourceCPU), int64(resourceMemory))
	resourceQuota2                    = forgeResourceList(int64(resourceCPU/2), int64(resourceMemory/2))
	resourceQuota4                    = forgeResourceList(int64(resourceCPU/4), int64(resourceMemory/4))
	foreignCluster                    = forgeForeignCluster(clusterID)
	foreignCluster2                   = forgeForeignCluster(clusterID2)
	quota                             = forgeQuotaWithLabel(tenantNamespace, string(clusterID), userName)
	quota2                            = forgeQuotaWithLabel(tenantNamespace2, string(clusterID2), userName2)
	fakeShadowPod                     = forgeShadowPod(nsName.Name, nsName.Namespace, string(testShadowPodUID), userName)
	fakeShadowPod2                    = forgeShadowPod(nsName2.Name, nsName2.Namespace, string(testShadowPodUID2), userName)
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
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(liqov1beta1.AddToScheme(scheme)).To(Succeed())
	Expect(offloadingv1beta1.AddToScheme(scheme)).To(Succeed())
})

func TestShadowpod(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shadowpod Suite")
}

func serializeShadowPod(sp *offloadingv1beta1.ShadowPod) runtime.RawExtension {
	data, err := json.Marshal(sp)
	Expect(err).ToNot(HaveOccurred())
	return runtime.RawExtension{Raw: data}
}

func forgeRequest(op admissionv1.Operation, newShadowPod, oldShadowPod *offloadingv1beta1.ShadowPod) admission.Request {
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

func forgeShadowPodWithClusterID(clusterID liqov1beta1.ClusterID, userName, namespace string) *offloadingv1beta1.ShadowPod {
	return &offloadingv1beta1.ShadowPod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testShadowPodName,
			Namespace: namespace,
			Labels: map[string]string{
				forge.LiqoOriginClusterIDKey: string(clusterID),
				consts.CreatorLabelKey:       userName,
			},
		},
	}
}

func forgeShadowPod(name, namespace, uid, creatorName string) *offloadingv1beta1.ShadowPod {
	return &offloadingv1beta1.ShadowPod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uid),
			Labels: map[string]string{
				consts.CreatorLabelKey: creatorName,
			},
		},
		Spec: offloadingv1beta1.ShadowPodSpec{
			Pod: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-container",
						Image: "test-image",
						Resources: corev1.ResourceRequirements{
							Requests: *forgeResourceList(int64(resourceCPU/4), int64(resourceMemory/4)),
						},
					},
				},
			},
		},
	}
}

func forgeShadowPodWithResourceRequests(containers, initContainer []containerResource) *offloadingv1beta1.ShadowPod {
	sp := &offloadingv1beta1.ShadowPod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testShadowPodName,
			Namespace: testNamespace,
			UID:       testShadowPodUID,
			Labels: map[string]string{
				forge.LiqoOriginClusterIDKey: string(clusterID),
				consts.CreatorLabelKey:       userName,
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
					Requests: *forgeResourceList(container.cpu, container.memory),
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
					Requests: *forgeResourceList(container.cpu, container.memory),
				},
			}
		}
	}
	return sp
}

func forgeShadowPodList(shadowPods ...*offloadingv1beta1.ShadowPod) *offloadingv1beta1.ShadowPodList {
	spList := &offloadingv1beta1.ShadowPodList{}

	for _, sp := range shadowPods {
		spList.Items = append(spList.Items, *sp)
	}

	return spList
}

func forgeQuotaWithLabel(namespace, clusterID, userName string) *offloadingv1beta1.Quota {
	q := &offloadingv1beta1.Quota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userName,
			Namespace: namespace,
		},
		Spec: offloadingv1beta1.QuotaSpec{
			User:      userName,
			Resources: *forgeResourceList(int64(resourceCPU), int64(resourceMemory)),
		},
	}
	if clusterID != "" && userName != "" {
		q.Labels = map[string]string{
			consts.RemoteClusterID:             clusterID,
			consts.ReplicationDestinationLabel: clusterID,
			consts.ReplicationRequestedLabel:   "true",
			consts.CreatorLabelKey:             userName,
		}
	}
	return q
}

func forgeForeignCluster(clusterID liqov1beta1.ClusterID) *liqov1beta1.ForeignCluster {
	return &liqov1beta1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(clusterID),
			Labels: map[string]string{
				consts.RemoteClusterID: string(clusterID),
			},
		},
		Spec: liqov1beta1.ForeignClusterSpec{
			ClusterID: clusterID,
		},
	}
}
