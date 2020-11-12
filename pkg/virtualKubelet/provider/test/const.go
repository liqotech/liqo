package test

import (
	"github.com/liqotech/liqo/internal/liqonet/tunnelEndpointCreator"
	pkg "github.com/liqotech/liqo/pkg/virtualKubelet"
	"time"
)

const (
	Namespace             = "test"
	NattedNamespace       = Namespace + "-" + HomeClusterId
	HostName              = "testHost"
	NodeName              = "testNode"
	AdvName               = pkg.AdvertisementPrefix + ForeignClusterId
	TepName               = tunnelEndpointCreator.TunEndpointNamePrefix + ForeignClusterId
	EndpointsName         = "testEndpoints"
	HomeClusterId         = "homeClusterID"
	ForeignClusterId      = "foreignClusterID"
	PodCIDR               = "1.2.3.4/16"
	RemoteRemappedPodCIDR = "5.6.7.8/16"
	TunnelPublicIP        = "5.6.7.8"
	LocalRemappedPodCIDR  = "100.200.0.0/16"
	Timeout               = 10 * time.Second
)
