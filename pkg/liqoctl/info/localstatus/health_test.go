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
//

package localstatus_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/info/localstatus"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("HealthChecker tests", func() {

	type TestArgs struct {
		totalPods int
		readyPods int
	}

	const unhealthyRestartCount = 10
	var (
		clientBuilder    fake.ClientBuilder
		hc               *localstatus.HealthChecker
		ctx              context.Context
		options          info.Options
		healthyPodStatus = corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "fake-container",
					RestartCount: 0,
					Ready:        false,
				},
			},
		}
		unhealthyPodStatus = corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionFalse,
				}},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "fake-container",
					RestartCount: unhealthyRestartCount,
					Ready:        false,
				},
			},
		}
	)

	getPodName := func(n int) string {
		return pterm.Sprintf("liqo-pod-%d", n)
	}

	BeforeEach(func() {
		ctx = context.Background()
		clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)

		options = info.Options{Factory: factory.NewForLocal()}
		options.Printer = output.NewFakePrinter(GinkgoWriter)
		options.KubeClient = k8sfake.NewSimpleClientset()
	})

	Describe("Testing the HealthChecker", func() {
		Context("Collecting and retrieving the data", func() {
			DescribeTable("should collect the data and return the right result", func(args TestArgs) {

				// Set up the fake clients creating some fake Liqo pods
				nUnhealthyPods := args.totalPods - args.readyPods
				shouldBeHealthy := args.totalPods == args.readyPods
				podsList := []client.Object{}

				for i := range args.totalPods {
					pod := testutil.FakePodWithSingleContainer(
						liqoconsts.DefaultLiqoNamespace,
						getPodName(i),
						"fake-image",
					)
					if nUnhealthyPods > 0 && i < nUnhealthyPods {
						pod.Status = unhealthyPodStatus
					} else {
						pod.Status = healthyPodStatus
					}
					podsList = append(podsList, pod)
				}

				clientBuilder.WithObjects(podsList...)
				options.CRClient = clientBuilder.Build()
				options.LiqoNamespace = liqoconsts.DefaultLiqoNamespace

				By("Collecting the data")
				hc = &localstatus.HealthChecker{}
				hc.Collect(ctx, options)

				By("Verifying that no errors have been raised")
				Expect(hc.GetCollectionErrors()).To(BeEmpty())

				By("Checking the correctness of the data in the struct")
				data := hc.GetData().(localstatus.Health)

				Expect(data.Healthy).To(Equal(shouldBeHealthy), "Unexpected installation status")
				if !shouldBeHealthy {
					Expect(data.UnhealthyPods).NotTo(BeEmpty(), "Installation is unhealthy but unhealthy pod list is empty")

					for i := range nUnhealthyPods {
						currentPod := data.UnhealthyPods[getPodName(i)]
						Expect(currentPod.TotalContainers).To(Equal(1))

						restarts := 0
						readyContainers := 1
						podStatus := healthyPodStatus.Phase
						if nUnhealthyPods > 0 && i < nUnhealthyPods {
							restarts = unhealthyRestartCount
							readyContainers = 0
							podStatus = unhealthyPodStatus.Phase
						}

						Expect(currentPod.Restarts).To(Equal(int32(restarts)), "Not matching restarts")
						Expect(currentPod.ReadyContainers).To(Equal(readyContainers), "Not matching ready containers")
						Expect(currentPod.Status).To(Equal(podStatus), "Not matching pod statuses")
					}
				}

				By("Checking the formatted output")
				text := hc.Format(options)
				text = pterm.RemoveColorFromString(text)
				text = testutil.SqueezeWhitespaces(text)

				if shouldBeHealthy {
					Expect(text).To(ContainSubstring("Liqo is healthy"))
				} else {
					Expect(text).To(ContainSubstring("Liqo is unhealthy"))
					for i := range nUnhealthyPods {
						Expect(text).To(ContainSubstring(
							pterm.Sprintf(
								"%s: Status: Pending, Ready: 0/1, Restarts: %d",
								getPodName(i),
								unhealthyRestartCount,
							),
						))
					}
				}
			},
				Entry("Healthy installation", TestArgs{
					totalPods: 3,
					readyPods: 3,
				}),
				Entry("Unhealthy installation", TestArgs{
					totalPods: 3,
					readyPods: 2,
				}),
				Entry("Unhealthy installation - No pods running", TestArgs{
					totalPods: 3,
					readyPods: 0,
				}),
			)
		})
	})
})
