package responsetypes

import (
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
)

// SigningRequestResponseType indicates the type for a signign request response.
type SigningRequestResponseType string

const (
	// SigningRequestResponseCertificate indicates that the signing request response contains a certificate
	// issued by the cluster CA.
	SigningRequestResponseCertificate SigningRequestResponseType = "Certificate"
	// SigningRequestResponseIAM indicates that the identity has been validated by the Amazon IAM service.
	SigningRequestResponseIAM SigningRequestResponseType = "IAM"
)

// AwsIdentityResponse contains the information about the created IAM user and the EKS cluster.
type AwsIdentityResponse struct {
	IamUserArn string
	AccessKey  *iam.AccessKey
	EksCluster *eks.Cluster
	Region     string
}

// SigningRequestResponse contains the response from an Indentity Provider.
type SigningRequestResponse struct {
	ResponseType SigningRequestResponseType

	Certificate []byte

	AwsIdentityResponse AwsIdentityResponse
}
