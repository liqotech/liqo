// Copyright 2019-2022 The Liqo Authors
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

package eks

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"k8s.io/klog/v2"
)

// createIamIdentity crates the Liqo IAM user identity.
func (k *eksProvider) createIamIdentity(sess *session.Session) error {
	iamSvc := iam.New(sess, aws.NewConfig().WithRegion(k.region))

	if err := k.ensureUser(iamSvc); err != nil {
		return err
	}

	policyArn, err := k.ensurePolicy(iamSvc)
	if err != nil {
		return err
	}

	attachUserPolicyRequest := &iam.AttachUserPolicyInput{
		PolicyArn: aws.String(policyArn),
		UserName:  aws.String(k.iamLiqoUser.userName),
	}

	_, err = iamSvc.AttachUserPolicy(attachUserPolicyRequest)
	if err != nil {
		return err
	}

	return nil
}

func (k *eksProvider) requiresUserCreation() bool {
	return k.iamLiqoUser.accessKeyID == "" || k.iamLiqoUser.secretAccessKey == ""
}

func (k *eksProvider) ensureUser(iamSvc *iam.IAM) error {
	if !k.requiresUserCreation() {
		valid, err := k.checkCredentials(iamSvc)
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("no accessKeyID %v found in the IAM user %v", k.iamLiqoUser.accessKeyID, k.iamLiqoUser.userName)
		}

		klog.V(3).Info("Using provided IAM credentials")
		return nil
	}

	createUserRequest := &iam.CreateUserInput{
		UserName: aws.String(k.iamLiqoUser.userName),
	}

	_, err := iamSvc.CreateUser(createUserRequest)
	if err != nil {
		return err
	}

	createAccessKeyRequest := &iam.CreateAccessKeyInput{
		UserName: aws.String(k.iamLiqoUser.userName),
	}

	createAccessKeyResult, err := iamSvc.CreateAccessKey(createAccessKeyRequest)
	if err != nil {
		return err
	}

	k.iamLiqoUser.accessKeyID = *createAccessKeyResult.AccessKey.AccessKeyId
	k.iamLiqoUser.secretAccessKey = *createAccessKeyResult.AccessKey.SecretAccessKey

	if err = storeIamAccessKey(k.iamLiqoUser.userName,
		k.iamLiqoUser.accessKeyID,
		k.iamLiqoUser.secretAccessKey); err != nil {
		return err
	}

	return nil
}

func (k *eksProvider) checkCredentials(iamSvc *iam.IAM) (bool, error) {
	listAccessKeysRequest := &iam.ListAccessKeysInput{
		UserName: aws.String(k.iamLiqoUser.userName),
	}

	listAccessKeysResult, err := iamSvc.ListAccessKeys(listAccessKeysRequest)
	if err != nil {
		return false, err
	}

	for i := range listAccessKeysResult.AccessKeyMetadata {
		accessKey := listAccessKeysResult.AccessKeyMetadata[i]
		if *accessKey.AccessKeyId == k.iamLiqoUser.accessKeyID {
			return true, nil
		}
	}

	return false, nil
}

func (k *eksProvider) ensurePolicy(iamSvc *iam.IAM) (string, error) {
	policyDocument, err := getPolicyDocument()
	if err != nil {
		return "", err
	}

	createPolicyRequest := &iam.CreatePolicyInput{
		PolicyName:     aws.String(k.iamLiqoUser.policyName),
		PolicyDocument: aws.String(policyDocument),
	}

	createPolicyResult, err := iamSvc.CreatePolicy(createPolicyRequest)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok { // nolint:errorlint // we need to access methods of the aws error interface
			switch aerr.Code() {
			case iam.ErrCodeEntityAlreadyExistsException:
				return k.checkPolicy(iamSvc)
			default:
				return "", err
			}
		} else {
			// not an AWS error
			return "", err
		}
	}

	return *createPolicyResult.Policy.Arn, nil
}

func (k *eksProvider) getPolicyArn(iamSvc *iam.IAM) (string, error) {
	getUserResult, err := iamSvc.GetUser(&iam.GetUserInput{})
	if err != nil {
		return "", err
	}

	splits := strings.Split(*getUserResult.User.Arn, ":")
	accountID := splits[4]

	return fmt.Sprintf("arn:aws:iam::%v:policy/%v", accountID, k.iamLiqoUser.policyName), nil
}

// checkPolicy checks that the retrieved policy has the required permission.
func (k *eksProvider) checkPolicy(iamSvc *iam.IAM) (string, error) {
	arn, err := k.getPolicyArn(iamSvc)
	if err != nil {
		return "", err
	}

	getPolicyRequest := &iam.GetPolicyInput{
		PolicyArn: aws.String(arn),
	}

	getPolicyResult, err := iamSvc.GetPolicy(getPolicyRequest)
	if err != nil {
		return "", err
	}
	defaultVersionID := getPolicyResult.Policy.DefaultVersionId

	getPolicyVersionRequest := &iam.GetPolicyVersionInput{
		PolicyArn: aws.String(arn),
		VersionId: defaultVersionID,
	}

	getPolicyVersionResult, err := iamSvc.GetPolicyVersion(getPolicyVersionRequest)
	if err != nil {
		return "", err
	}

	policyDocument, err := getPolicyDocument()
	if err != nil {
		return "", err
	}

	tmp, err := url.QueryUnescape(*getPolicyVersionResult.PolicyVersion.Document)
	if err != nil {
		return "", err
	}

	if tmp != policyDocument {
		return "", fmt.Errorf("the %v IAM policy has not the permission required by Liqo",
			k.iamLiqoUser.policyName)
	}

	return arn, nil
}
