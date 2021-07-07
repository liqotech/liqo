package testconsts

const (
	// NumberOfTestClusters number of clusters used in E2E tests.
	NumberOfTestClusters = 4

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
)
