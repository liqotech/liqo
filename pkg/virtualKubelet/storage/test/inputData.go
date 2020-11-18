package test

import (
	"github.com/liqotech/liqo/pkg/virtualKubelet/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	HomeNamespace    = "homeNamespace"
	ForeignNamespace = "foreignNamespace"

	Pod1 = "homePod1"
	Pod2 = "homePod2"
)

var (
	Pods = map[string]*corev1.Pod{
		utils.Keyer(HomeNamespace, Pod1): {
			ObjectMeta: metav1.ObjectMeta{
				Name:      Pod1,
				Namespace: HomeNamespace,
			},
		},
		utils.Keyer(HomeNamespace, Pod2): {
			ObjectMeta: metav1.ObjectMeta{
				Name:      Pod2,
				Namespace: HomeNamespace,
			},
		},

		utils.Keyer(ForeignNamespace, Pod1): {
			ObjectMeta: metav1.ObjectMeta{
				Name:      Pod1,
				Namespace: ForeignNamespace,
			},
		},
		utils.Keyer(ForeignNamespace, Pod2): {
			ObjectMeta: metav1.ObjectMeta{
				Name:      Pod2,
				Namespace: ForeignNamespace,
			},
		},
	}
)
