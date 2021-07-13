package testconsts

import "github.com/liqotech/liqo/pkg/consts"

const (
	liqoTestingLabelKey = "liqo.io/testing-namespace"
)

// LiqoTestNamespaceLabels is a set of labels that has to be attached to test namespaces to simplify garbage collection.
var LiqoTestNamespaceLabels = map[string]string{
	liqoTestingLabelKey:      "true",
	consts.EnablingLiqoLabel: consts.EnablingLiqoLabelValue,
}

const (
	// Keys for cluster labels.

	// ProviderKey indicates the cluster provider.
	ProviderKey = "provider"
	// RegionKey indicates the cluster region.
	RegionKey = "region"

	// Values for cluster labels.

	// ProviderAzure -> provider=Azure.
	ProviderAzure = "Azure"
	// ProviderAWS -> provider=AWS.
	ProviderAWS = "AWS"
	// ProviderGKE -> provider=GKE.
	ProviderGKE = "GKE"
	// RegionA -> region=A.
	RegionA = "A"
	// RegionB -> region=B.
	RegionB = "B"
	// RegionC -> region=C.
	RegionC = "C"
	// RegionD -> region=D.
	RegionD = "D"

	// LiqoTestingLabelKey is a label that has to be attached to test namespaces to simplify garbage collection.
	LiqoTestingLabelKey = "liqo.io/testing-namespace"
	// LiqoTestingLabelValue is the value of the LiqoTestingLabelKey.
	LiqoTestingLabelValue = "true"
)
