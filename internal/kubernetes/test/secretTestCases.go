package test

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var SecretTestCases = struct {
	InputSecrets  map[string]*corev1.Secret
	UpdateSecrets map[string]*corev1.Secret
	DeleteSecrets map[string]*corev1.Secret
}{
	InputSecrets: map[string]*corev1.Secret{
		"secret1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret1",
				Namespace: Namespace,
			},
			StringData: map[string]string{
				"k1": "v1",
				"k2": "v2",
				"k3": "v3",
			},
		},
		"secret2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret2",
				Namespace: Namespace,
			},
			StringData: map[string]string{
				"k11": "v11",
				"k22": "v22",
			},
		},
	},
	UpdateSecrets: map[string]*corev1.Secret{
		"secret1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret1",
				Namespace: Namespace,
			},
			StringData: map[string]string{
				"ku1": "vu1",
				"k2":  "v2",
			},
		},
		"secret2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret2",
				Namespace: Namespace,
			},
			StringData: map[string]string{
				"ku11": "vu11",
				"k22":  "v22",
			},
		},
	},
	DeleteSecrets: map[string]*corev1.Secret{
		"secret2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret2",
				Namespace: Namespace,
			},
		},
	},
}
