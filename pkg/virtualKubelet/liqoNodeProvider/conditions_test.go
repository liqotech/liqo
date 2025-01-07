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

package liqonodeprovider

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Liqo node conditions generation", func() {
	Describe("The UnknownNodeConditions function", func() {
		When("generating the conditions", func() {
			var conditions []corev1.NodeCondition

			DescribeBody := func(conditionType corev1.NodeConditionType) func() {
				return func() {
					It("Should be present and have unknown status", func() {
						Expect(conditions).To(ContainElement(*unknownCondition(conditionType)))
					})
				}
			}

			JustBeforeEach(func() { conditions = UnknownNodeConditions(&InitConfig{CheckNetworkStatus: true}) })
			Describe("The NodeReady condition", DescribeBody(corev1.NodeReady))
			Describe("The NodeMemoryPressure condition", DescribeBody(corev1.NodeMemoryPressure))
			Describe("The NodeDiskPressure condition", DescribeBody(corev1.NodeDiskPressure))
			Describe("The NodePIDPressure condition", DescribeBody(corev1.NodePIDPressure))
			Describe("The NodeNetworkUnavailable condition", DescribeBody(corev1.NodeNetworkUnavailable))
		})
	})

	Describe("The UpdateNodeCondition function", func() {
		var (
			node     corev1.Node
			modified *corev1.NodeCondition
			status   corev1.ConditionStatus
		)

		BeforeEach(func() {
			node = corev1.Node{Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeNetworkUnavailable}},
			}}
			status = corev1.ConditionTrue
		})
		JustBeforeEach(func() {
			updater := func() (corev1.ConditionStatus, string, string) {
				return status, "reason", "message"
			}
			UpdateNodeCondition(&node, corev1.NodeReady, updater)
			modified = lookupCondition(&node, corev1.NodeReady)
		})

		DescribeBodyUpdated := func() func() {
			return func() {
				It("Should be present", func() { Expect(modified).ToNot(BeNil()) })
				It("Should have the correct status", func() { Expect(modified.Status).To(Equal(status)) })
				It("Should have the correct reason", func() { Expect(modified.Reason).To(Equal("reason")) })
				It("Should have the correct message", func() { Expect(modified.Message).To(Equal("message")) })
				It("Should have the heartbeat time updated", func() {
					Expect(modified.LastHeartbeatTime.Time).To(BeTemporally("~", time.Now(), time.Second))
				})
				It("Should have the transition time updated", func() {
					Expect(modified.LastTransitionTime.Time).To(BeTemporally("~", time.Now(), time.Second))
				})
			}
		}

		DescribeBodyUnchanged := func() func() {
			return func() {
				It("Should be present", func() { Expect(modified).ToNot(BeNil()) })
				It("Should have the correct status", func() { Expect(modified.Status).To(Equal(status)) })
				It("Should have the unmodified reason", func() { Expect(modified.Reason).To(Equal("previous reason")) })
				It("Should have the unmodified message", func() { Expect(modified.Message).To(Equal("previous message")) })
				It("Should have the heartbeat time updated", func() {
					Expect(modified.LastHeartbeatTime.Time).To(BeTemporally("~", time.Now(), time.Second))
				})
				It("Should have the transition time unchanged", func() {
					Expect(modified.LastTransitionTime.Time).To(BeTemporally("~", time.Now().Add(-1*time.Hour), time.Second))
				})
			}
		}

		When("the condition does not yet exist", func() {
			Describe("updating the condition", DescribeBodyUpdated())
		})

		When("the condition already exists", func() {
			BeforeEach(func() {
				node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
					Type:               corev1.NodeReady,
					Status:             corev1.ConditionTrue,
					Reason:             "previous reason",
					Message:            "previous message",
					LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-1 * time.Hour)),
					LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
				})
			})

			When("it already has the correct status", func() {
				Describe("updating the condition", DescribeBodyUnchanged())
			})

			When("it has incorrect status", func() {
				BeforeEach(func() { status = corev1.ConditionFalse })
				Describe("updating the condition", DescribeBodyUpdated())
			})
		})
	})

	Describe("Node conditions status generation", func() {
		type StatusGenerationCase struct {
			Generator       func() (status corev1.ConditionStatus, reason, message string)
			ExpectedStatus  corev1.ConditionStatus
			ExpectedReason  string
			ExpectedMessage string
		}

		DescribeTable("Check correct status generation",
			func(c StatusGenerationCase) {
				status, reason, message := c.Generator()
				Expect(status).To(BeIdenticalTo(c.ExpectedStatus))
				Expect(reason).To(BeIdenticalTo(c.ExpectedReason))
				Expect(message).To(BeIdenticalTo(c.ExpectedMessage))
			},
			Entry("of the ready condition, when ready", StatusGenerationCase{
				Generator:       nodeReadyStatus(true),
				ExpectedStatus:  corev1.ConditionTrue,
				ExpectedReason:  "KubeletReady",
				ExpectedMessage: "The Liqo Virtual Kubelet is posting ready status",
			}),
			Entry("of the ready condition, when not ready", StatusGenerationCase{
				Generator:       nodeReadyStatus(false),
				ExpectedStatus:  corev1.ConditionFalse,
				ExpectedReason:  "KubeletNotReady",
				ExpectedMessage: "The Liqo Virtual Kubelet is currently not ready",
			}),
			Entry("of the memory pressure condition, when set", StatusGenerationCase{
				Generator:       nodeMemoryPressureStatus(true),
				ExpectedStatus:  corev1.ConditionTrue,
				ExpectedReason:  "RemoteClusterHasMemoryPressure",
				ExpectedMessage: "The remote cluster is advertising no/insufficient resources",
			}),
			Entry("of the memory pressure condition, when unset", StatusGenerationCase{
				Generator:       nodeMemoryPressureStatus(false),
				ExpectedStatus:  corev1.ConditionFalse,
				ExpectedReason:  "RemoteClusterHasSufficientMemory",
				ExpectedMessage: "The remote cluster is advertising sufficient resources",
			}),
			Entry("of the disk pressure condition, when set", StatusGenerationCase{
				Generator:       nodeDiskPressureStatus(true),
				ExpectedStatus:  corev1.ConditionTrue,
				ExpectedReason:  "RemoteClusterHasDiskPressure",
				ExpectedMessage: "The remote cluster is advertising no/insufficient resources",
			}),
			Entry("of the disk pressure condition, when unset", StatusGenerationCase{
				Generator:       nodeDiskPressureStatus(false),
				ExpectedStatus:  corev1.ConditionFalse,
				ExpectedReason:  "RemoteClusterHasNoDiskPressure",
				ExpectedMessage: "The remote cluster is advertising sufficient resources",
			}),
			Entry("of the PID pressure condition, when set", StatusGenerationCase{
				Generator:       nodePIDPressureStatus(true),
				ExpectedStatus:  corev1.ConditionTrue,
				ExpectedReason:  "RemoteClusterHasPIDPressure",
				ExpectedMessage: "The remote cluster is advertising no/insufficient resources",
			}),
			Entry("of the PID pressure condition, when unset", StatusGenerationCase{
				Generator:       nodePIDPressureStatus(false),
				ExpectedStatus:  corev1.ConditionFalse,
				ExpectedReason:  "RemoteClusterHasNoPIDPressure",
				ExpectedMessage: "The remote cluster is advertising sufficient resources",
			}),
			Entry("of the network unavailable condition, when set", StatusGenerationCase{
				Generator:       nodeNetworkUnavailableStatus(true),
				ExpectedStatus:  corev1.ConditionTrue,
				ExpectedReason:  "LiqoNetworkingDown",
				ExpectedMessage: "The Liqo cluster interconnection is down",
			}),
			Entry("of the network unavailable condition, when unset", StatusGenerationCase{
				Generator:       nodeNetworkUnavailableStatus(false),
				ExpectedStatus:  corev1.ConditionFalse,
				ExpectedReason:  "LiqoNetworkingUp",
				ExpectedMessage: "The Liqo cluster interconnection is established",
			}),
		)
	})

	Describe("Checking the unknownCondition function", func() {
		When("generating a new node condition", func() {
			var (
				condition *corev1.NodeCondition
				desired   corev1.NodeConditionType
			)

			JustBeforeEach(func() { condition = unknownCondition(desired) })
			It("Should have the desired type", func() { Expect(condition.Type).To(BeIdenticalTo(desired)) })
			It("Should have unknown status", func() { Expect(condition.Status).To(BeIdenticalTo(corev1.ConditionUnknown)) })
		})
	})

	Describe("Checking the lookupCondition function", func() {
		var (
			node    corev1.Node
			desired corev1.NodeCondition
		)

		BeforeEach(func() {
			desired = corev1.NodeCondition{Type: corev1.NodeReady}
			node = corev1.Node{Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{desired, {Type: corev1.NodeMemoryPressure}},
			}}
		})

		When("the condition exists", func() {
			It("should return the expected value", func() {
				Expect(*lookupCondition(&node, corev1.NodeReady)).To(Equal(desired))
			})
		})

		When("the condition does not exist", func() {
			It("should return return nil", func() {
				Expect(lookupCondition(&node, corev1.NodeNetworkUnavailable)).To(BeNil())
			})
		})
	})

	Describe("Checking the lookupConditionOrCreateUnknown function", func() {
		var (
			node               corev1.Node
			key                corev1.NodeConditionType
			desired, retrieved *corev1.NodeCondition
			found              bool
		)

		BeforeEach(func() {
			desired = &corev1.NodeCondition{Type: corev1.NodeReady}
			node = corev1.Node{Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{*desired, {Type: corev1.NodeMemoryPressure}},
			}}
		})

		JustBeforeEach(func() {
			retrieved, found = lookupConditionOrCreateUnknown(&node, key)
		})

		When("the condition exists", func() {
			BeforeEach(func() { key = corev1.NodeReady })
			It("should return the expected value", func() { Expect(*retrieved).To(Equal(*desired)) })
			It("should return a found value equal to true", func() { Expect(found).To(BeTrue()) })
		})

		When("the condition does not exist", func() {
			BeforeEach(func() { key = corev1.NodeNetworkUnavailable })
			It("should return a new condition", func() { Expect(*retrieved).To(Equal(*unknownCondition(key))) })
			It("should return a found value equal to false", func() { Expect(found).To(BeFalse()) })
		})
	})
})
