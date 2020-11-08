package test

import corev1 "k8s.io/api/core/v1"

func AssertServiceCoherency(svc1, svc2 corev1.Service) bool {
	if svc1.Name != svc2.Name {
		return false
	}

	for _, p1 := range svc1.Spec.Ports {
		var found bool
		for _, p2 := range svc2.Spec.Ports {
			if p1.Name == p2.Name {
				if p1.Protocol == p2.Protocol &&
					p1.NodePort == p2.NodePort &&
					p1.Port == p2.Port &&
					p1.TargetPort == p2.TargetPort {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}

	return true
}
