package identitymanager

// AwsConfig contains the AWS configuration and access key for the Liqo user and the current EKS cluster.
type AwsConfig struct {
	AwsAccessKeyID     string
	AwsSecretAccessKey string
	AwsRegion          string
	AwsClusterName     string
}

// IsEmpty indicates that some of the required values is not set.
func (ac *AwsConfig) IsEmpty() bool {
	return ac == nil || ac.AwsAccessKeyID == "" || ac.AwsSecretAccessKey == "" || ac.AwsRegion == "" || ac.AwsClusterName == ""
}
