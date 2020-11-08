package test

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ConfigmapTestCases = struct {
	InputConfigmaps  map[string]*corev1.ConfigMap
	UpdateConfigmaps map[string]*corev1.ConfigMap
	DeleteConfigmaps map[string]*corev1.ConfigMap
}{
	InputConfigmaps: map[string]*corev1.ConfigMap{
		"configmap1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap1",
				Namespace: Namespace,
			},
			Data: map[string]string{
				"k1": "v1",
				"k2": "v2",
				"k3": "v3",
			},
		},
		"configmap2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap2",
				Namespace: Namespace,
			},
			Data: map[string]string{
				"k11": "v11",
				"k22": "v22",
			},
		},
	},
	UpdateConfigmaps: map[string]*corev1.ConfigMap{
		"configmap1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap1",
				Namespace: Namespace,
			},
			Data: map[string]string{
				"ku1": "vu1",
				"k2":  "v2",
			},
		},
		"configmap2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap2",
				Namespace: Namespace,
			},
			Data: map[string]string{
				"ku11": "vu11",
				"k22":  "v22",
			},
		},
	},
	DeleteConfigmaps: map[string]*corev1.ConfigMap{
		"configmap2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap2",
				Namespace: Namespace,
			},
		},
	},
}
