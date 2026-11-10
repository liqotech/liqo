// Copyright 2019-2026 The Liqo Authors
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

package pod

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	netutils "k8s.io/utils/net"
)

var (
	// ErrNoIPAddress is returned when the pod has no IP address assigned yet.
	// Callers should retry once the pod status is updated.
	ErrNoIPAddress = errors.New("pod has no IP address assigned yet")
	// ErrNoIPv4Address is returned when the pod has IP addresses assigned, but none of them is IPv4
	// (e.g., IPv6 single-stack clusters). The Liqo network fabric is IPv4 only, so the pod cannot be reached through it.
	ErrNoIPv4Address = errors.New("pod has no IPv4 address")
)

// IPv4 returns the IPv4 address of the given pod.
// See IPv4FromStatus for details.
func IPv4(pod *corev1.Pod) (string, error) {
	return IPv4FromStatus(&pod.Status)
}

// IPv4FromStatus returns the IPv4 address of a pod, given its status.
// The Liqo network fabric is IPv4 only. On dual-stack clusters whose primary IP family is IPv6,
// status.podIP is the IPv6 address, while the IPv4 address is only listed in status.podIPs.
// Hence, it returns the first IPv4 address in status.podIPs, falling back to status.podIP if it is IPv4.
// It returns ErrNoIPAddress if the pod has no IP address yet,
// and ErrNoIPv4Address if the pod has IP addresses, but none of them is IPv4.
func IPv4FromStatus(status *corev1.PodStatus) (string, error) {
	for i := range status.PodIPs {
		if netutils.IsIPv4String(status.PodIPs[i].IP) {
			return status.PodIPs[i].IP, nil
		}
	}

	if netutils.IsIPv4String(status.PodIP) {
		return status.PodIP, nil
	}

	if status.PodIP == "" && len(status.PodIPs) == 0 {
		return "", ErrNoIPAddress
	}
	return "", ErrNoIPv4Address
}
