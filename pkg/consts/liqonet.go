package consts

const (
	// NetworkManagerIpamPort is the port used by IPAM gRPCs.
	NetworkManagerIpamPort = 6000
	// NetworkManagerServiceName is the service name for IPAM gRPCs.
	NetworkManagerServiceName = "liqo-network-manager"
	// DefaultCIDRValue is the default value for a string that contains a CIDR.
	DefaultCIDRValue = "None"
	// TepReady is the ready state of TunnelEndpoint resource.
	TepReady = "Ready"
	// NatMappingKind constant is used as Kind value for CR NatMapping.
	NatMappingKind = "NatMapping"
	// NatMappingResourceLabelKey constant is used for label of resource.
	NatMappingResourceLabelKey = "net.liqo.io/natmapping"
	// NatMappingResourceLabelValue constant is used for label of resource.
	NatMappingResourceLabelValue = "true"
	// IpamStorageResourceLabelKey constant is used for label of resource.
	IpamStorageResourceLabelKey = "net.liqo.io/ipamstorage"
	// IpamStorageResourceLabelValue constant is used for label of resource.
	IpamStorageResourceLabelValue = "true"
)
