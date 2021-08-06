package pod_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/liqotech/liqo/pkg/utils/pod"
)

var _ = Describe("Pod utility functions", func() {

	Describe("The IsPodReady function", func() {
		type IsPodReadyCase struct {
			Pod      *corev1.Pod
			Expected bool
		}

		PodGenerator := func(status corev1.ConditionStatus) *corev1.Pod {
			return &corev1.Pod{
				Status: corev1.PodStatus{Conditions: []corev1.PodCondition{
					{Type: "foo", Status: corev1.ConditionFalse},
					{Type: "bar", Status: corev1.ConditionTrue},
					{Type: corev1.PodReady, Status: status},
				}},
			}
		}

		PodGeneratorWithoutConditions := func() *corev1.Pod {
			return &corev1.Pod{}
		}

		DescribeTable("Should return the correct output",
			func(c IsPodReadyCase) {
				Expect(pod.IsPodReady(c.Pod)).To(BeIdenticalTo(c.Expected))
			},
			Entry("When the pod is ready", IsPodReadyCase{Pod: PodGenerator(corev1.ConditionTrue), Expected: true}),
			Entry("When the pod is not ready", IsPodReadyCase{Pod: PodGenerator(corev1.ConditionFalse), Expected: false}),
			Entry("When the pod has no conditions", IsPodReadyCase{Pod: PodGeneratorWithoutConditions(), Expected: false}),
		)
	})
})
