package liqonodeprovider

import (
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

// NewLiqoNodeProvider creates and returns a new LiqoNodeProvider.
func NewLiqoNodeProvider(
	nodeName, foreignClusterID, kubeletNamespace string,
	node *v1.Node,
	podProviderStopper, networkReadyChan chan struct{},
	config *rest.Config, resyncPeriod time.Duration) (*LiqoNodeProvider, error) {
	if config == nil {
		config = ctrl.GetConfigOrDie()
	}
	client := kubernetes.NewForConfigOrDie(config)
	dynClient := dynamic.NewForConfigOrDie(config)

	return &LiqoNodeProvider{
		client:    client,
		dynClient: dynClient,

		node:              node,
		terminating:       false,
		lastAppliedLabels: map[string]string{},

		networkReady:       false,
		podProviderStopper: podProviderStopper,
		networkReadyChan:   networkReadyChan,
		resyncPeriod:       resyncPeriod,

		nodeName:         nodeName,
		foreignClusterID: foreignClusterID,
		kubeletNamespace: kubeletNamespace,
	}, nil
}
