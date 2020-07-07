package test

import "time"

const (
	Namespace        = "test"
	NattedNamespace  = Namespace + "-" + HomeClusterId
	HostName         = "testHost"
	NodeName         = "testNode"
	AdvName          = "advertisement-" + ForeignClusterId
	EndpointsName    = "testEndpoints"
	HomeClusterId    = "homeClusterID"
	ForeignClusterId = "foreignClusterID"
	Timeout          = 10 * time.Second
)
