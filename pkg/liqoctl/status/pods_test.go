// Copyright 2019-2022 The Liqo Authors
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

package status

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"
)

var _ = Describe("Pods", func() {
	Describe("CollectionError", func() {
		var (
			appType = "Deployment"
			appName = "app"
			err     = errors.New("ce")
		)

		Context("creating a new collectionError", func() {
			It("should hold the passed parameters during the creation", func() {
				ce := newCollectionError(appType, appName, err)
				Expect(ce.appType).To(Equal(appType))
				Expect(ce.appName).To(Equal(appName))
				Expect(ce.err).To(MatchError(err))
			})
		})
	})

	Describe("ComponentState", func() {
		var (
			deploymentType = "Deployment"
			image          = "liqo:testImage"
			podS           componentState
		)
		JustBeforeEach(func() {
			podName := "pod1"
			err := errors.New("pod1")
			podS = componentState{
				errorFlag:      false,
				controllerType: "",
				desired:        1,
				ready:          1,
				available:      1,
				unavailable:    0,
				imageVersions:  []string{image},
				errors:         errorCountMap{podName: &errorCount{[]error{err}}},
			}
		})

		Describe("creating a new componentState", func() {
			It("should hold the passed parameters during the creation", func() {
				ps := newComponentState(deploymentType)
				Expect(ps.controllerType).To(Equal(deploymentType))
			})
		})

		Describe("handling images field", func() {
			Context("getting images", func() {
				It("should return a slice of length 1", func() {
					im := podS.getImages()
					Expect(im).To(HaveLen(1))
					Expect(im[0]).To(Equal(image))
				})
			})

			Context("setting images", func() {
				It("should set the new images", func() {
					newImages := []string{"image1", "image2"}
					podS.setImages(newImages)
					Expect(len(podS.imageVersions)).To(BeNumerically("==", 2))
					Expect(podS.imageVersions).To(Equal(newImages))
				})
			})

			Context("adding an image to the existing ones", func() {
				When("the image does not exist", func() {
					It("should append the image to the existing ones", func() {
						newImage := "newImage"
						podS.addImageVersion(newImage)
						Expect(len(podS.imageVersions)).To(BeNumerically("==", 2))
						Expect(podS.imageVersions[1]).To(Equal(newImage))
					})
				})

				When("the image exists", func() {
					It("should not append the image", func() {
						podS.addImageVersion(image)
						Expect(len(podS.imageVersions)).To(BeNumerically("==", 1))
					})
				})
			})
		})

		Describe("handling errorsPerPod field", func() {
			var (
				podName = "pod1"
				err     = errors.New("new podName error")
			)

			Context("adding error per podName", func() {
				When("no errors per podName exists", func() {
					It("should add the error for the given podName", func() {
						podS.errors = errorCountMap{}
						podS.addErrorForPod(podName, err)
						Expect(podS.errors[podName].errors).To(HaveLen(1))
						Expect(podS.errors[podName].errors[0]).To(MatchError(err))
					})
				})

				When("errors per podName exists", func() {
					It("should append the new error for the given podName", func() {
						podS.addErrorForPod(podName, err)
						Expect(podS.errors[podName].errors).To(HaveLen(2))
						Expect(podS.errors[podName].errors[1]).To(MatchError(err))
					})
				})
			})
		})

		Describe("formatting the componentState", func() {
			When("there are no unavailable pods", func() {
				It("string should not contain unavailable pods", func() {
					Expect(podS.format()).NotTo(ContainSubstring("Unavailable"))
				})

			})

			When("not all pods are ready", func() {
				It("output should contain unavailable pods", func() {
					podS.desired = 2
					podS.unavailable = 1
					Expect(podS.format()).To(ContainSubstring(fmt.Sprintf(" Unavailable: " + red + "1/2" + reset)))
				})
			})
		})
	})

	Describe("podChecker", func() {
		var (
			pod           *v1.Pod
			deployment    *appsv1.Deployment
			daemonSet     *appsv1.DaemonSet
			podC          podChecker
			ctx           = context.Background()
			namespace     = "namespaceTest"
			deploymentApp = "deploymentTest"
			deployments   = []string{deploymentApp}
			daemonSetApp  = "daemonSetTest"
			daemonSets    = []string{daemonSetApp}

			depLabels = map[string]string{
				"app": deploymentApp,
			}

			dsLabels = map[string]string{
				"app": daemonSetApp,
			}
		)
		BeforeEach(func() {
			pod = &v1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: namespace,
					Labels:    depLabels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
				},
			}

			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentApp,
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32Ptr(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: depLabels,
					},
				},
			}

			daemonSet = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      daemonSetApp,
					Namespace: namespace,
				},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: dsLabels,
					},
				},
			}
		})
		JustBeforeEach(func() {
			podC = podChecker{
				deployments:      deployments,
				daemonSets:       daemonSets,
				namespace:        namespace,
				name:             checkerName,
				podsState:        make(podStateMap, 2),
				errors:           false,
				collectionErrors: nil,
			}
		})

		Describe("creating a new podChecker", func() {
			It("should hold the passed parameters during the creation", func() {
				pc := newPodChecker(namespace, deployments, daemonSets, fake.NewSimpleClientset())
				Expect(pc.namespace).To(Equal(namespace))
				Expect(pc.daemonSets).To(Equal(daemonSets))
				Expect(pc.deployments).To(Equal(deployments))
				Expect(pc.client).NotTo(BeNil())
				Expect(pc.errors).To(BeFalse())
			})
		})

		Describe("Collect() function", func() {
			Context("collecting deployment apps", func() {
				When("fails to get the deployment", func() {
					It("should add an error to the collectionErrors", func() {
						podC.client = fake.NewSimpleClientset()
						Expect(podC.Collect(ctx)).To(BeNil())
						Expect(podC.errors).To(BeTrue())
						Expect(podC.collectionErrors).To(HaveLen(2))
					})
				})

				When("fails to get the pod related to the deployment", func() {
					It("should add an error to the collectionErrors", func() {
						podC.client = fake.NewSimpleClientset(deployment)
						Expect(podC.Collect(ctx)).To(BeNil())
						Expect(podC.errors).To(BeTrue())
						Expect(podC.collectionErrors).To(HaveLen(2))
					})
				})

				When("deployment and pod exist", func() {
					It("should not add errors", func() {
						pod.SetLabels(depLabels)
						podC.client = fake.NewSimpleClientset(deployment, pod)
						Expect(podC.Collect(ctx)).To(BeNil())
						Expect(podC.errors).To(BeTrue())
						Expect(podC.collectionErrors).To(HaveLen(1))
					})
				})
			})

			Context("collecting daemonSets apps", func() {
				When("fails to get the daemonSet", func() {
					It("should add an error to the collectionErrors", func() {
						podC.client = fake.NewSimpleClientset()
						Expect(podC.Collect(ctx)).To(BeNil())
						Expect(podC.errors).To(BeTrue())
						Expect(podC.collectionErrors).To(HaveLen(2))
					})
				})

				When("fails to get the pod related to the daemonSet", func() {
					It("should add an error to the collectionErrors", func() {
						podC.client = fake.NewSimpleClientset(daemonSet)
						Expect(podC.Collect(ctx)).To(BeNil())
						Expect(podC.errors).To(BeTrue())
						Expect(podC.collectionErrors).To(HaveLen(2))
					})
				})

				When("daemonSet and pod exist", func() {
					It("should not add errors", func() {
						pod.SetLabels(dsLabels)
						podC.client = fake.NewSimpleClientset(daemonSet, pod)
						Expect(podC.Collect(ctx)).To(BeNil())
						Expect(podC.errors).To(BeTrue())
						Expect(podC.collectionErrors).To(HaveLen(1))
					})
				})
			})
		})

		Describe("deploymentStatus() function", func() {
			When("fails to get the deployment", func() {
				It("should return an error", func() {
					podC.client = fake.NewSimpleClientset()
					Expect(podC.deploymentStatus(ctx, deploymentApp)).To(HaveOccurred())
				})
			})

			When("fails to get the pod related to the deployment", func() {
				It("should return an error", func() {
					podC.client = fake.NewSimpleClientset(deployment)
					Expect(podC.deploymentStatus(ctx, deploymentApp)).To(HaveOccurred())
				})
			})

			When("deployment and pod exist", func() {
				It("should return nil", func() {
					pod.SetLabels(depLabels)
					podC.client = fake.NewSimpleClientset(deployment, pod)
					Expect(podC.deploymentStatus(ctx, deploymentApp)).NotTo(HaveOccurred())
				})
			})
		})

		Describe("daemonSetStatus() function", func() {
			When("fails to get the daemonSet", func() {
				It("should return an error", func() {
					podC.client = fake.NewSimpleClientset()
					Expect(podC.daemonSetStatus(ctx, daemonSetApp)).To(HaveOccurred())
				})
			})

			When("fails to get the pod related to the daemonSet", func() {
				It("should return an error", func() {
					podC.client = fake.NewSimpleClientset(daemonSet)
					Expect(podC.daemonSetStatus(ctx, daemonSetApp)).To(HaveOccurred())
				})
			})

			When("daemonSet and pod exist", func() {
				It("should return nil", func() {
					pod.SetLabels(dsLabels)
					podC.client = fake.NewSimpleClientset(daemonSet, pod)
					Expect(podC.daemonSetStatus(ctx, daemonSetApp)).NotTo(HaveOccurred())
				})
			})
		})

		Describe("Format() function", func() {
			When("status is not ok", func() {
				It("should state that the status is not OK", func() {
					podC.errors = true
					msg, err := podC.Format()
					Expect(err).NotTo(HaveOccurred())
					Expect(msg).To(ContainSubstring(fmt.Sprintf("%s liqo control plane is not OK\n", redCross)))
				})
			})

			When("status is ok", func() {
				It("should state that the status is OK", func() {
					msg, err := podC.Format()
					Expect(err).NotTo(HaveOccurred())
					Expect(msg).To(ContainSubstring(fmt.Sprintf("%s control plane pods are up and running\n", checkMark)))
				})
			})
		})

		Describe("HasSucceeded() function", func() {
			When("check succeeds", func() {
				It("should return true", func() {
					podC.errors = false
					Expect(podC.HasSucceeded()).To(BeTrue())
				})
			})

			When("check fails", func() {
				It("should return false", func() {
					podC.errors = true
					Expect(podC.HasSucceeded()).To(BeFalse())
				})
			})
		})

		Describe("checkPodsStatus() function", func() {

			var (
				pod1 = &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "podTest",
					},
					Status: v1.PodStatus{
						Phase: "",
						Conditions: []v1.PodCondition{
							{
								Type:   v1.PodInitialized,
								Status: v1.ConditionFalse,
							}},
					},
				}

				pod2 = &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "podTest",
					},
					Status: v1.PodStatus{
						Phase: "",
						Conditions: []v1.PodCondition{
							{
								Type:   v1.PodScheduled,
								Status: v1.ConditionFalse,
							}},
					},
				}

				pod3 = &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "podTest",
					},
					Status: v1.PodStatus{
						Phase: "",
						Conditions: []v1.PodCondition{
							{
								Type:   v1.PodReady,
								Status: v1.ConditionFalse,
							}},
					},
				}
			)

			DescribeTable("checking all cases",
				func(p *v1.Pod, expectedErr error, expectedBool bool) {
					podsList := []v1.Pod{*p}
					dState := newComponentState("testing")
					Expect(checkPodsStatus(podsList, &dState)).To(Equal(expectedBool))
					if expectedErr != nil {
						Expect(dState.errors[p.Name].errors[0]).To(MatchError(expectedErr))
					} else {
						Expect(dState.errors[p.Name].errors[0]).To(BeNil())
					}
				},
				Entry("pod is not initialized", pod1, fmt.Errorf("not initialized"), true),
				Entry("pod is not scheduled", pod2, fmt.Errorf("not scheduled"), true),
				Entry("pod is not ready", pod3, fmt.Errorf("not ready"), true),
			)
		})
	})

})
