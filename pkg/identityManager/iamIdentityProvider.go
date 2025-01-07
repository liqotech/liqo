// Copyright 2019-2025 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package identitymanager

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

const (
	namespaceKubeSystem      = "kube-system"
	awsAuthConfigMapName     = "aws-auth"
	awsAuthConfigMapUsersKey = "mapUsers"
)

type iamIdentityProvider struct {
	localAwsConfig *LocalAwsConfig
	cl             client.Client
	localClusterID string
}

type mapUser struct {
	UserArn  string   `json:"userarn"`
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
}

func (identityProvider *iamIdentityProvider) init(ctx context.Context) error {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(identityProvider.localAwsConfig.AwsRegion),
		Credentials: credentials.NewStaticCredentialsFromCreds(credentials.Value{
			AccessKeyID:     identityProvider.localAwsConfig.AwsAccessKeyID,
			SecretAccessKey: identityProvider.localAwsConfig.AwsSecretAccessKey,
		}),
	})
	if err != nil {
		klog.Error(err)
		return err
	}

	eksCluster, err := identityProvider.getEksClusterInfo(ctx, sess)
	if err != nil {
		klog.Error(err)
		return err
	}

	identityProvider.localAwsConfig.AwsClusterEndpoint = *eksCluster.Endpoint
	identityProvider.localAwsConfig.AwsClusterCA = []byte(*eksCluster.CertificateAuthority.Data)

	return nil
}

func (identityProvider *iamIdentityProvider) GetRemoteCertificate(ctx context.Context,
	options *SigningRequestOptions) (response *responsetypes.SigningRequestResponse, err error) {
	response = &responsetypes.SigningRequestResponse{
		ResponseType: responsetypes.SigningRequestResponseIAM,
	}
	secretName := remoteCertificateSecretName(options)
	var secret corev1.Secret
	if err := identityProvider.cl.Get(ctx, types.NamespacedName{
		Namespace: options.TenantNamespace,
		Name:      secretName,
	}, &secret); err != nil {
		if kerrors.IsNotFound(err) {
			klog.V(4).Info(err)
		} else {
			klog.Error(err)
		}
		return response, err
	}

	response = &responsetypes.SigningRequestResponse{
		ResponseType: responsetypes.SigningRequestResponseIAM,
		AwsIdentityResponse: responsetypes.AwsIdentityResponse{
			IamUserArn:                         string(secret.Data[AwsIAMUserArnSecretKey]),
			AccessKeyID:                        string(secret.Data[AwsAccessKeyIDSecretKey]),
			SecretAccessKey:                    string(secret.Data[AwsSecretAccessKeySecretKey]),
			EksClusterName:                     identityProvider.localAwsConfig.AwsClusterName,
			EksClusterEndpoint:                 identityProvider.localAwsConfig.AwsClusterEndpoint,
			EksClusterCertificateAuthorityData: identityProvider.localAwsConfig.AwsClusterCA,
			Region:                             identityProvider.localAwsConfig.AwsRegion,
		},
	}
	return response, nil
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)

func clearString(str string) string {
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}

func (identityProvider *iamIdentityProvider) ApproveSigningRequest(ctx context.Context,
	options *SigningRequestOptions) (response *responsetypes.SigningRequestResponse, err error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(identityProvider.localAwsConfig.AwsRegion),
		Credentials: credentials.NewStaticCredentialsFromCreds(credentials.Value{
			AccessKeyID:     identityProvider.localAwsConfig.AwsAccessKeyID,
			SecretAccessKey: identityProvider.localAwsConfig.AwsSecretAccessKey,
		}),
	})
	if err != nil {
		klog.Error(err)
		return response, err
	}

	iamSvc := iam.New(sess)

	var username string
	var organization string

	switch options.IdentityType {
	case authv1beta1.ControlPlaneIdentityType:
		username = authentication.CommonNameControlPlaneCSR(options.Cluster)
		organization = authentication.OrganizationControlPlaneCSR()
	case authv1beta1.ResourceSliceIdentityType:
		if options.ResourceSlice == nil {
			klog.Error("resource slice is nil")
			return response, fmt.Errorf("resource slice is nil")
		}

		username = authentication.CommonNameResourceSliceCSR(options.ResourceSlice)
		organization = authentication.OrganizationResourceSliceCSR(options.ResourceSlice)
	default:
		klog.Errorf("identity type %v not supported", options.IdentityType)
		return response, fmt.Errorf("identity type %v not supported", options.IdentityType)
	}

	// the IAM username has to have <= 64 characters
	s := fmt.Sprintf("%s-%s-%s", username, organization, identityProvider.localClusterID)
	h := sha256.New()
	_, _ = h.Write([]byte(s))
	bs := h.Sum(nil)
	bsStr := clearString(base64.StdEncoding.EncodeToString(bs))
	iamUsername := fmt.Sprintf("liqo-%s", bsStr)
	if len(iamUsername) > 63 {
		iamUsername = iamUsername[:63]
	}
	klog.Infof("IAM username: %s", iamUsername)
	tags := map[string]string{
		localClusterIDTagKey:  identityProvider.localClusterID,
		remoteClusterIDTagKey: string(options.Cluster),
		managedByTagKey:       managedByTagValue,
		identityTypeTagKey:    string(options.IdentityType),
	}
	if options.IdentityType == authv1beta1.ResourceSliceIdentityType {
		tags[consts.ResourceSliceNameLabelKey] = options.ResourceSlice.Name
	}

	userArn, err := identityProvider.ensureIamUser(ctx, iamSvc, iamUsername, tags)
	if err != nil {
		klog.Error(err)
		return response, err
	}

	accessKey, err := identityProvider.ensureIamAccessKey(ctx, iamSvc, iamUsername)
	if err != nil {
		klog.Error(err)
		return response, err
	}

	if err = identityProvider.ensureConfigMap(ctx, userArn, username, organization); err != nil {
		klog.Error(err)
		return response, err
	}

	if _, err = identityProvider.storeRemoteCertificate(ctx,
		*accessKey.AccessKeyId, *accessKey.SecretAccessKey, userArn,
		options); err != nil {
		klog.Error(err)
		return response, err
	}

	return &responsetypes.SigningRequestResponse{
		ResponseType: responsetypes.SigningRequestResponseIAM,
		AwsIdentityResponse: responsetypes.AwsIdentityResponse{
			IamUserArn:                         userArn,
			AccessKeyID:                        *accessKey.AccessKeyId,
			SecretAccessKey:                    *accessKey.SecretAccessKey,
			EksClusterName:                     identityProvider.localAwsConfig.AwsClusterName,
			EksClusterEndpoint:                 identityProvider.localAwsConfig.AwsClusterEndpoint,
			EksClusterCertificateAuthorityData: identityProvider.localAwsConfig.AwsClusterCA,
			Region:                             identityProvider.localAwsConfig.AwsRegion,
		},
	}, nil
}

func (identityProvider *iamIdentityProvider) ForgeAuthParams(ctx context.Context,
	options *SigningRequestOptions) (*authv1beta1.AuthParams, error) {
	resp, err := EnsureCertificate(ctx, identityProvider, options)
	if err != nil {
		return nil, err
	}

	var ca []byte
	if options.CAOverride != nil {
		ca = options.CAOverride
	} else {
		ca = resp.AwsIdentityResponse.EksClusterCertificateAuthorityData
	}

	var apiServer string
	if options.APIServerAddressOverride != "" {
		apiServer = options.APIServerAddressOverride
	} else {
		apiServer = resp.AwsIdentityResponse.EksClusterEndpoint
	}

	return &authv1beta1.AuthParams{
		CA:        ca,
		APIServer: apiServer,
		AwsConfig: &authv1beta1.AwsConfig{
			AwsUserArn:         resp.AwsIdentityResponse.IamUserArn,
			AwsAccessKeyID:     resp.AwsIdentityResponse.AccessKeyID,
			AwsSecretAccessKey: resp.AwsIdentityResponse.SecretAccessKey,
			AwsRegion:          resp.AwsIdentityResponse.Region,
			AwsClusterName:     resp.AwsIdentityResponse.EksClusterName,
		},
	}, nil
}

func (identityProvider *iamIdentityProvider) ensureIamUser(ctx context.Context,
	iamSvc *iam.IAM, username string, tags map[string]string) (string, error) {
	iamTags := make([]*iam.Tag, len(tags))
	i := 0
	for k, v := range tags {
		iamTags[i] = &iam.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		}
		i++
	}

	createUser := &iam.CreateUserInput{
		UserName: aws.String(username),
		Tags:     iamTags,
	}

	createUserResult, err := iamSvc.CreateUserWithContext(ctx, createUser)
	if err != nil {
		// ignore already exists error
		if aerr, ok := err.(awserr.Error); ok { //nolint:errorlint // aws does not export a specific error type
			if aerr.Code() == iam.ErrCodeEntityAlreadyExistsException {
				klog.Warningf("IAM user %v already exists, Liqo will crate a new access key for it", username)

				user, err := identityProvider.getUser(ctx, iamSvc, username)
				if err != nil {
					klog.Error(err)
					return "", err
				}

				return *user.Arn, nil
			}
		}

		klog.Error(err)
		return "", err
	}

	return *createUserResult.User.Arn, nil
}

func (identityProvider *iamIdentityProvider) getUser(ctx context.Context,
	iamSvc *iam.IAM, username string) (*iam.User, error) {
	getUserInput := &iam.GetUserInput{
		UserName: aws.String(username),
	}

	getUserOutput, err := iamSvc.GetUserWithContext(ctx, getUserInput)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return getUserOutput.User, nil
}

func (identityProvider *iamIdentityProvider) ensureIamAccessKey(ctx context.Context,
	iamSvc *iam.IAM, username string) (*iam.AccessKey, error) {
	createAccessKey := &iam.CreateAccessKeyInput{
		UserName: aws.String(username),
	}

	createAccessKeyResult, err := iamSvc.CreateAccessKeyWithContext(ctx, createAccessKey)
	if err == nil {
		return createAccessKeyResult.AccessKey, nil
	}

	// if the error is limit exceeded, we have to delete an existing access key
	if aerr, ok := err.(awserr.Error); ok { //nolint:errorlint // aws does not export a specific error type
		if aerr.Code() == iam.ErrCodeLimitExceededException {
			klog.Warningf("IAM user %v has reached the limit of access keys, Liqo will delete an existing access key", username)
			var accessKeyList *iam.ListAccessKeysOutput
			accessKeyList, err = iamSvc.ListAccessKeysWithContext(ctx, &iam.ListAccessKeysInput{
				UserName: aws.String(username),
			})
			if err != nil {
				klog.Error(err)
				return nil, err
			}

			if len(accessKeyList.AccessKeyMetadata) == 0 {
				klog.Error("no access key found")
				return nil, fmt.Errorf("no access key found")
			}
			for _, accessKey := range accessKeyList.AccessKeyMetadata {
				_, err = iamSvc.DeleteAccessKeyWithContext(ctx, &iam.DeleteAccessKeyInput{
					AccessKeyId: accessKey.AccessKeyId,
					UserName:    aws.String(username),
				})
				if err != nil {
					klog.Error(err)
					return nil, err
				}
			}

			createAccessKeyResult, err = iamSvc.CreateAccessKeyWithContext(ctx, createAccessKey)
		}
	}

	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return createAccessKeyResult.AccessKey, nil
}

func (identityProvider *iamIdentityProvider) getEksClusterInfo(ctx context.Context,
	sess *session.Session) (*eks.Cluster, error) {
	eksSvc := eks.New(sess)

	describeCluster := &eks.DescribeClusterInput{
		Name: aws.String(identityProvider.localAwsConfig.AwsClusterName),
	}

	describeClusterResult, err := eksSvc.DescribeClusterWithContext(ctx, describeCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return describeClusterResult.Cluster, nil
}

func (identityProvider *iamIdentityProvider) ensureConfigMap(ctx context.Context, userArn, username, organization string) error {
	authCm := corev1.ConfigMap{}
	err := identityProvider.cl.Get(ctx, types.NamespacedName{
		Namespace: namespaceKubeSystem,
		Name:      awsAuthConfigMapName,
	}, &authCm)
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
		klog.V(4).Infof("the map %v already contains userARN %s (user: %s, organization: %s)", awsAuthConfigMapName, userArn, username, organization)
		return nil
	}

	users = append(users, mapUser{
		UserArn:  userArn,
		Username: username,
		Groups: []string{
			organization,
		},
	})

	bytes, err := yaml.Marshal(users)
	if err != nil {
		klog.Error(err)
		return err
	}

	authCm.Data[awsAuthConfigMapUsersKey] = string(bytes)
	err = identityProvider.cl.Update(ctx, &authCm)
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

// storeRemoteCertificate stores the issued certificate in a Secret in the TenantNamespace.
func (identityProvider *iamIdentityProvider) storeRemoteCertificate(ctx context.Context,
	accessKeyID, secretAccessKey, userArn string,
	options *SigningRequestOptions) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      remoteCertificateSecretName(options),
			Namespace: options.TenantNamespace,
		},
	}

	_, err := resource.CreateOrUpdate(ctx, identityProvider.cl, secret, func() error {
		if secret.Labels == nil {
			secret.Labels = map[string]string{}
		}
		secret.Labels[consts.RemoteClusterID] = string(options.Cluster)

		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		secret.Data[csrSecretKey] = options.SigningRequest
		secret.Data[AwsAccessKeyIDSecretKey] = []byte(accessKeyID)
		secret.Data[AwsSecretAccessKeySecretKey] = []byte(secretAccessKey)
		secret.Data[AwsIAMUserArnSecretKey] = []byte(userArn)

		return nil
	})
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return secret, nil
}
