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
	nodeName, advName, foreignClusterID, kubeletNamespace string,
	node *v1.Node,
	podProviderStopper, networkReadyChan chan struct{},
	config *rest.Config, resyncPeriod time.Duration, useNewAuth bool) (*LiqoNodeProvider, error) {
	if config == nil {
		config = ctrl.GetConfigOrDie()
	}
	client := kubernetes.NewForConfigOrDie(config)
	dynClient := dynamic.NewForConfigOrDie(config)

	return &LiqoNodeProvider{
		client:    client,
		dynClient: dynClient,

		node:              node,
		lastAppliedLabels: map[string]string{},

		networkReady:       false,
		podProviderStopper: podProviderStopper,
		networkReadyChan:   networkReadyChan,
		resyncPeriod:       resyncPeriod,

		nodeName:         nodeName,
		advName:          advName,
		foreignClusterID: foreignClusterID,
		kubeletNamespace: kubeletNamespace,

		useNewAuth: useNewAuth,
	}, nil
}
