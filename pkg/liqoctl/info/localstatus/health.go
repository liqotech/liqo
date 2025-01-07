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

package localstatus

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	podutils "github.com/liqotech/liqo/pkg/utils/pod"
)

// PodHealthInfo represents the current status of a Liqo pod.
type PodHealthInfo struct {
	Status          corev1.PodPhase `json:"status"`
	ReadyContainers int             `json:"readyContainers"`
	TotalContainers int             `json:"totalContainers"`
	Restarts        int32           `json:"restarts"`
}

// Health represents the current status of a Liqo instance.
type Health struct {
	// Healthy is true whenever all the Liqo pods are up and running.
	Healthy       bool                     `json:"healthy"`
	UnhealthyPods map[string]PodHealthInfo `json:"unhealthyPods,omitempty"`
}

// HealthChecker collects the info about the local instance of Liqo.
type HealthChecker struct {
	info.CheckerCommon
	data Health
}

// Collect data about the health of the local instance of Liqo.
func (l *HealthChecker) Collect(ctx context.Context, options info.Options) {
	var liqoPodsList corev1.PodList
	if err := options.CRClient.List(ctx, &liqoPodsList, client.InNamespace(options.LiqoNamespace)); err != nil {
		l.AddCollectionError(fmt.Errorf("unable to get Liqo pods: %w", err))
		return
	}

	// Check the status of each pods
	l.data.Healthy = true
	l.data.UnhealthyPods = map[string]PodHealthInfo{}
	liqoPods := liqoPodsList.Items
	for i := range liqoPods {
		pod := &liqoPods[i]
		// Check if one of the pods is not ready, in that case declare Liqo installation as Unhealthy
		if ok, _ := podutils.IsPodReady(pod); !ok {
			l.data.Healthy = false
			healthInfo := PodHealthInfo{
				Status:          pod.Status.Phase,
				TotalContainers: len(pod.Status.ContainerStatuses),
			}

			// Populate the info about restarts and running containers
			for i := range pod.Status.ContainerStatuses {
				containerStatus := &pod.Status.ContainerStatuses[i]
				healthInfo.Restarts += containerStatus.RestartCount
				if containerStatus.Ready {
					healthInfo.ReadyContainers++
				}
			}
			l.data.UnhealthyPods[pod.Name] = healthInfo
		}
	}
}

// Format returns the collected data using a user friendly output.
func (l *HealthChecker) Format(options info.Options) string {
	main := output.NewRootSection()
	if l.data.Healthy {
		main.AddSectionSuccess(fmt.Sprintf("%s    Liqo is healthy", output.CheckMark))
	} else {
		main.AddSectionFailure(fmt.Sprintf("%s    Liqo is unhealthy", output.Cross))
	}

	if len(l.data.UnhealthyPods) > 0 {
		podsSection := main.AddSection("Unhealthy pods")
		for podName, podInfo := range l.data.UnhealthyPods {
			podsSection.AddEntryWithoutStyle(
				podName,
				fmt.Sprintf("Status: %v, Ready: %v/%v, Restarts: %v", podInfo.Status, podInfo.ReadyContainers, podInfo.TotalContainers, podInfo.Restarts),
			)
		}
	}

	return main.SprintForBox(options.Printer)
}

// GetData returns the data collected by the checker.
func (l *HealthChecker) GetData() interface{} {
	return l.data
}

// GetID returns the id of the section collected by the checker.
func (l *HealthChecker) GetID() string {
	return "health"
}

// GetTitle returns the title of the section collected by the checker.
func (l *HealthChecker) GetTitle() string {
	return "Installation health"
}
