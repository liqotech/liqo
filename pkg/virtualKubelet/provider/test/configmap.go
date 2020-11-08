package test

import corev1 "k8s.io/api/core/v1"

func AssertConfigmapCoherency(cm1, cm2 corev1.ConfigMap) bool {
	if cm1.Name != cm2.Name {
		return false
	}

	for k1, v1 := range cm1.Data {
		v2, ok := cm2.Data[k1]

		if !ok || v1 != v2 {
			return false
		}
	}

	for k1, v2 := range cm2.Data {
		v1, ok := cm1.Data[k1]

		if !ok || v1 != v2 {
			return false
		}
	}

	return true
}
