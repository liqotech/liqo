package consts

const (
	// LiqonetPostroutingChain is the name of the postrouting chain inserted by liqo.
	LiqonetPostroutingChain = "LIQO-POSTROUTING"
	// LiqonetPreroutingChain is the naame of the prerouting chain inserted by liqo.
	LiqonetPreroutingChain = "LIQO-PREROUTING"
	// LiqonetForwardingChain is the name of the forwarding chain inserted by liqo.
	LiqonetForwardingChain = "LIQO-FORWARD"
	// LiqonetInputChain is the name of the input chain inserted by liqo.
	LiqonetInputChain = "LIQO-INPUT"
	// LiqonetPostroutingClusterChainPrefix the prefix used to name the postrouting chains for a specific cluster.
	LiqonetPostroutingClusterChainPrefix = "LIQO-PSTRT-CLS-"
	// LiqonetPreroutingClusterChainPrefix prefix used to name the prerouting chains for a specific cluster.
	LiqonetPreroutingClusterChainPrefix = "LIQO-PRRT-CLS-"
	// LiqonetForwardingClusterChainPrefix prefix used to name the forwarding chains for a specific cluster.
	LiqonetForwardingClusterChainPrefix = "LIQO-FRWD-CLS-"
	// LiqonetInputClusterChainPrefix prefix used to name the input chains for a specific cluster.
	LiqonetInputClusterChainPrefix = "LIQO-INPT-CLS-"
	// NatTable constant used for the "nat" table.
	NatTable = "nat"
	// FilterTable constant used for the "filter" table.
	FilterTable = "filter"
	// PreroutingChain constant.
	PreroutingChain = "PREROUTING"
	// PostroutingChain constant.
	PostroutingChain = "POSTROUTING"
	// InputChain constant.
	InputChain = "INPUT"
	// ForwardChain constant.
	ForwardChain = "FORWARD"
	// MASQUERADE action constant.
	MASQUERADE = "MASQUERADE"
	// SNAT action constant.
	SNAT = "SNAT"
	// NETMAP action constant.
	NETMAP = "NETMAP"
	// ACCEPT action constant.
	ACCEPT = "ACCEPT"
	// LocalNATPodCIDR constant is used for errors.
	LocalNATPodCIDR = "LocalNATPodCIDR"
	// RemotePodCIDR constant is used for errors.
	RemotePodCIDR = "RemotePodCIDR"
	// LocalPodCIDR constant is used for errors.
	LocalPodCIDR = "LocalPodCIDR"
)
