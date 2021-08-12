package identitymanager

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"gopkg.in/yaml.v3"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
)

const (
	namespaceKubeSystem      = "kube-system"
	awsAuthConfigMapName     = "aws-auth"
	awsAuthConfigMapUsersKey = "mapUsers"
)

type iamIdentityProvider struct {
	awsConfig *AwsConfig
	client    kubernetes.Interface
}

type mapUser struct {
	UserArn  string   `json:"userarn"`
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
}

func (identityProvider *iamIdentityProvider) GetRemoteCertificate(clusterID, namespace,
	signingRequest string) (response *responsetypes.SigningRequestResponse, err error) {
	// this method has no meaning for this identity provider
	return response, kerrors.NewNotFound(schema.GroupResource{
		Group:    "v1",
		Resource: "secrets",
	}, remoteCertificateSecret)
}

func (identityProvider *iamIdentityProvider) ApproveSigningRequest(clusterID,
	signingRequest string) (response *responsetypes.SigningRequestResponse, err error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(identityProvider.awsConfig.AwsRegion),
		Credentials: credentials.NewStaticCredentialsFromCreds(credentials.Value{
			AccessKeyID:     identityProvider.awsConfig.AwsAccessKeyID,
			SecretAccessKey: identityProvider.awsConfig.AwsSecretAccessKey,
		}),
	})
	if err != nil {
		klog.Error(err)
		return response, err
	}

	iamSvc := iam.New(sess)

	userArn, err := identityProvider.ensureIamUser(sess, iamSvc, clusterID)
	if err != nil {
		klog.Error(err)
		return response, err
	}

	accessKey, err := identityProvider.ensureIamAccessKey(sess, iamSvc, clusterID)
	if err != nil {
		klog.Error(err)
		return response, err
	}

	eksCluster, err := identityProvider.getEksClusterInfo(sess)
	if err != nil {
		klog.Error(err)
		return response, err
	}

	if err = identityProvider.ensureConfigMap(userArn, clusterID); err != nil {
		klog.Error(err)
		return response, err
	}

	return &responsetypes.SigningRequestResponse{
		ResponseType: responsetypes.SigningRequestResponseIAM,
		AwsIdentityResponse: responsetypes.AwsIdentityResponse{
			IamUserArn: userArn,
			AccessKey:  accessKey,
			EksCluster: eksCluster,
			Region:     identityProvider.awsConfig.AwsRegion,
		},
	}, nil
}

func (identityProvider *iamIdentityProvider) ensureIamUser(sess *session.Session, iamSvc *iam.IAM, clusterID string) (string, error) {
	createUser := &iam.CreateUserInput{
		UserName: aws.String(clusterID),
	}

	createUserResult, err := iamSvc.CreateUser(createUser)
	if err != nil {
		// if the IAM user already exists, we cannot create another access key, since the previous creation
		// can be made from another cluster. We have to validate a secret from the remote cluster before to continue
		klog.Error(err)
		return "", err
	}

	return *createUserResult.User.Arn, nil
}

func (identityProvider *iamIdentityProvider) ensureIamAccessKey(sess *session.Session, iamSvc *iam.IAM, clusterID string) (*iam.AccessKey, error) {
	createAccessKey := &iam.CreateAccessKeyInput{
		UserName: aws.String(clusterID),
	}

	createAccessKeyResult, err := iamSvc.CreateAccessKey(createAccessKey)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return createAccessKeyResult.AccessKey, nil
}

func (identityProvider *iamIdentityProvider) getEksClusterInfo(sess *session.Session) (*eks.Cluster, error) {
	eksSvc := eks.New(sess)

	describeCluster := &eks.DescribeClusterInput{
		Name: aws.String(identityProvider.awsConfig.AwsClusterName),
	}

	describeClusterResult, err := eksSvc.DescribeCluster(describeCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return describeClusterResult.Cluster, nil
}

func (identityProvider *iamIdentityProvider) ensureConfigMap(userArn, clusterID string) error {
	ctx := context.TODO()
	authCm, err := identityProvider.client.CoreV1().ConfigMaps(namespaceKubeSystem).Get(ctx, awsAuthConfigMapName, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}

	var users []mapUser
	err = yaml.Unmarshal([]byte(authCm.Data[awsAuthConfigMapUsersKey]), &users)
	if err != nil {
		klog.Error(err)
		return err
	}

	if containsUser(users, userArn) {
		klog.V(4).Infof("the map %v already contains user %v (clusterID: %v)", awsAuthConfigMapName, userArn, clusterID)
		return nil
	}

	users = append(users, mapUser{
		UserArn:  userArn,
		Username: clusterID,
		Groups: []string{
			defaultOrganization,
		},
	})

	bytes, err := yaml.Marshal(users)
	if err != nil {
		klog.Error(err)
		return err
	}

	authCm.Data[awsAuthConfigMapUsersKey] = string(bytes)
	_, err = identityProvider.client.CoreV1().ConfigMaps(namespaceKubeSystem).Update(ctx, authCm, metav1.UpdateOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

func containsUser(currentUsers []mapUser, userArn string) bool {
	for i := range currentUsers {
		if currentUsers[i].UserArn == userArn {
			return true
		}
	}
	return false
}
