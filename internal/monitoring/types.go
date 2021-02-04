package monitoring

type LiqoComponent int

const (
	VirtualKubelet LiqoComponent = iota
	ForeignBroadcaster
	ForeignClusterOperator
	PeeringRequestOperator
	AdvertisementOperator
	TunnelEndpointOperator

	// always keep this as the last liqo component
	lastComponent
)

func (l LiqoComponent) String() string {
	return [...]string{"VirtualKubelet",
		"ForeignBroadcaster",
		"ForeignClusterOperator",
		"PeeringRequestOperator",
		"AdvertisementOperator",
		"TunnelEndpointOperator"}[l]
}

type EventStatus int

const (
	Start EventStatus = iota
	End
)

func (l EventStatus) String() string {
	return [...]string{"Start", "End"}[l]
}

type EventType int

const (
	CreatePeeringRequest EventType = iota
	CheckNetworkConfigs
	CheckTunnelEndpoints
	CheckAdvertisement
	CheckPeeringRequest
	CreateBroadcaster
	CreateAdvertisement
	GetPeeringRequest
	CreateAdvertisementClient
	CreateVirtualKubelet
	WaitForAdvertisement
	WaitForTunnelEndpoint
	CreateVirtualNode
	CreateTunnelEndpoint
	ProcessLocalNetworkConfig
	ProcessRemoteNetworkConfig

	// always keep this as the last event type
	lastEvent
)

func (l EventType) String() string {
	return [...]string{"CreatePeeringRequest",
		"CheckNetworkConfigs",
		"CheckTunnelEndpoints",
		"CheckAdvertisement",
		"CheckPeeringRequest",
		"CreateBroadcaster",
		"CreateAdvertisement",
		"GetPeeringRequest",
		"CreateAdvertisementClient",
		"CreateVirtualKubelet",
		"WaitForAdvertisement",
		"WaitForTunnelEndpoint",
		"CreateVirtualNode",
		"CreateTunnelEndpoint",
		"ProcessLocalNetworkConfig",
		"ProcessRemoteNetworkConfig"}[l]
}
