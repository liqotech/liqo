package uninstaller

import (
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/vkMachinery"
)

const TickerInterval = 5 * time.Second
const TickerTimeout = 1 * time.Minute
const ConditionsToCheck = 1

type toCheckDeleted struct {
	gvr           schema.GroupVersionResource
	labelSelector metav1.LabelSelector
}

type resultType struct {
	Resource toCheckDeleted
	Success  bool
}

var (
	podGVR = v1.SchemeGroupVersion.WithResource("pods")

	toCheck = []toCheckDeleted{
		{
			gvr:           v1alpha1.TunnelEndpointGroupVersionResource,
			labelSelector: metav1.LabelSelector{},
		},
		{
			gvr:           v1alpha1.NetworkConfigGroupVersionResource,
			labelSelector: metav1.LabelSelector{},
		},
		{
			gvr: podGVR,
			labelSelector: metav1.LabelSelector{
				MatchLabels: vkMachinery.KubeletBaseLabels,
			},
		},
	}
)
