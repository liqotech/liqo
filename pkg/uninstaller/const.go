package uninstaller

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/liqotech/liqo/apis/net/v1alpha1"
)

// TickerInterval defines the check interval.
const TickerInterval = 5 * time.Second

// TickerTimeout defines the overall timeout to be waited.
const TickerTimeout = 5 * time.Minute

// ConditionsToCheck maps the number of conditions to be checked waiting for the unpeer.
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
	toCheck = []toCheckDeleted{
		{
			gvr:           v1alpha1.TunnelEndpointGroupVersionResource,
			labelSelector: metav1.LabelSelector{},
		},
		{
			gvr:           v1alpha1.NetworkConfigGroupVersionResource,
			labelSelector: metav1.LabelSelector{},
		},
	}
)
