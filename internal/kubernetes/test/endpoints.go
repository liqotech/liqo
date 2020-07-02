package test

import corev1 "k8s.io/api/core/v1"

func AssertEndpointsCoherency(received, expected []corev1.EndpointSubset) bool {
	if len(received) != len(expected) {
		return false
	}
	for i := 0; i < len(received); i++ {
		if len(received[i].Addresses) != len(expected[i].Addresses) {
			return false
		}
		for j := 0; j < len(received[i].Addresses); j++ {
			if received[i].Addresses[j].IP != expected[i].Addresses[j].IP {
				return false
			}
		}
	}

	return true
}
