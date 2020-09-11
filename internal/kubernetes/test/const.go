package test

import (
	controllers "github.com/liqotech/liqo/internal/liqonet"
	"time"
)

const (
	Namespace             = "test"
	NattedNamespace       = Namespace + "-" + HomeClusterId
	HostName              = "testHost"
	NodeName              = "testNode"
	AdvName               = "advertisement-" + ForeignClusterId
	TepName               = controllers.TunEndpointNamePrefix + ForeignClusterId
	EndpointsName         = "testEndpoints"
	HomeClusterId         = "homeClusterID"
	ForeignClusterId      = "foreignClusterID"
	PodCIDR               = "1.2.3.4/16"
	RemoteRemappedPodCIDR = "5.6.7.8/16"
	TunnelPublicIP        = "5.6.7.8"
	LocalRemappedPodCIDR  = "100.200.0.0/16"
	Timeout               = 10 * time.Second
)
