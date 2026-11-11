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

package service

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	netutils "k8s.io/utils/net"
)

// ErrNoIPv4ClusterIP is returned when a service has no IPv4 ClusterIP
// (e.g., IPv6 single-stack and headless services). The Liqo network fabric is IPv4 only,
// so such a service cannot be reached through it.
var ErrNoIPv4ClusterIP = errors.New("service has no IPv4 ClusterIP")

// IPv4ClusterIP returns the IPv4 ClusterIP of the given service.
// The Liqo network fabric is IPv4 only. On dual-stack clusters whose primary IP family is IPv6,
// spec.clusterIP is the IPv6 address, while the IPv4 address is only listed in spec.clusterIPs.
// Hence, it returns the first IPv4 address in spec.clusterIPs, falling back to spec.clusterIP if it is IPv4.
// It returns ErrNoIPv4ClusterIP if the service has no IPv4 ClusterIP.
func IPv4ClusterIP(svc *corev1.Service) (string, error) {
	for _, ip := range svc.Spec.ClusterIPs {
		if netutils.IsIPv4String(ip) {
			return ip, nil
		}
	}
	if netutils.IsIPv4String(svc.Spec.ClusterIP) {
		return svc.Spec.ClusterIP, nil
	}
	return "", ErrNoIPv4ClusterIP
}
