package test

import corev1 "k8s.io/api/core/v1"

func AssertSecretCoherency(s1, s2 corev1.Secret) bool {
	if s1.Name != s2.Name {
		return false
	}

	for k1, v1 := range s1.StringData {
		v2, ok := s2.StringData[k1]

		if !ok || v1 != v2 {
			return false
		}
	}

	for k1, v2 := range s2.StringData {
		v1, ok := s1.StringData[k1]

		if !ok || v1 != v2 {
			return false
		}
	}

	return true
}
