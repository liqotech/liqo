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
	// IpamStorageResourceLabelKey constant is used for label of resource.
	IpamStorageResourceLabelKey = "net.liqo.io/ipamstorage"
	// IpamStorageResourceLabelValue constant is used for label of resource.
	IpamStorageResourceLabelValue = "true"
)
