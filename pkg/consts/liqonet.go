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
	// NatMappingKind is the constant representing
	// the value of the Kind field of all NatMapping resources.
	NatMappingKind = "NatMapping"
	// NatMappingResourceLabelKey is the constant representing
	// the key of the label assigned to all NatMapping resources.
	NatMappingResourceLabelKey = "net.liqo.io/natmapping"
	// NatMappingResourceLabelValue is the constant representing
	// the value of the label assigned to all NatMapping resources.
	NatMappingResourceLabelValue = "true"
	// IpamStorageResourceLabelKey is the constant representing
	// the key of the label assigned to all IpamStorage resources.
	IpamStorageResourceLabelKey = "net.liqo.io/ipamstorage"
	// IpamStorageResourceLabelValue is the constant representing
	// the value of the label assigned to all IpamStorage resources.
	IpamStorageResourceLabelValue = "true"
)
