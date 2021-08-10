package provider

import (
	"context"

	"go.opencensus.io/trace"
	v1 "k8s.io/api/core/v1"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqonodeprovider "github.com/liqotech/liqo/pkg/virtualKubelet/liqoNodeProvider"
)

func (p *LiqoProvider) ConfigureNode(ctx context.Context, n *v1.Node) {
	_, span := trace.StartSpan(ctx, "kubernetes.ConfigureNode")
	defer span.End()

	n.Status.Capacity = v1.ResourceList{}
	n.Status.Allocatable = v1.ResourceList{}
	n.Status.Conditions = liqonodeprovider.UnknownNodeConditions()
	n.Status.Addresses = p.nodeAddresses()
	n.Status.DaemonEndpoints = p.nodeDaemonEndpoints()
	os := p.operatingSystem
	if os == "" {
		os = "Linux"
	}
	n.Status.NodeInfo.OperatingSystem = os
	n.Status.NodeInfo.Architecture = "amd64"
	n.ObjectMeta.Labels["alpha.service-controller.kubernetes.io/exclude-balancer"] = "true"
	n.ObjectMeta.Labels["node.kubernetes.io/exclude-from-external-load-balancers"] = "true"
	n.Labels[liqoconst.TypeLabel] = liqoconst.TypeNode
	if n.Annotations == nil {
		n.Annotations = map[string]string{}
	}
	n.Annotations[liqoconst.RemoteClusterID] = p.foreignClusterID
}

// NodeAddresses returns a list of addresses for the node status
// within Kubernetes.
func (p *LiqoProvider) nodeAddresses() []v1.NodeAddress {
	return []v1.NodeAddress{
		{
			Type:    "InternalIP",
			Address: p.internalIP,
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *LiqoProvider) nodeDaemonEndpoints() v1.NodeDaemonEndpoints {
	return v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}
