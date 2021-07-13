package testconsts

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

	// ClusterEnvVarKey is the key of the environment variable that indicates the number of clusters available for the
	// execution of e2e tests.
	ClusterEnvVarKey = "CLUSTERS"
)
