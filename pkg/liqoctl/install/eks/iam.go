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

package eks

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
)

// createIamIdentity crates the Liqo IAM user identity.
func (o *Options) createIamIdentity(sess *session.Session) error {
	iamSvc := iam.New(sess, aws.NewConfig().WithRegion(o.region))

	if err := o.ensureUser(iamSvc); err != nil {
		return err
	}

	policyArn, err := o.ensurePolicy(iamSvc)
	if err != nil {
		return err
	}

	attachUserPolicyRequest := &iam.AttachUserPolicyInput{
		PolicyArn: aws.String(policyArn),
		UserName:  aws.String(o.iamUser.userName),
	}

	_, err = iamSvc.AttachUserPolicy(attachUserPolicyRequest)
	if err != nil {
		return err
	}

	return nil
}

func (o *Options) requiresUserCreation() bool {
	return o.iamUser.accessKeyID == "" || o.iamUser.secretAccessKey == ""
}

func (o *Options) ensureUser(iamSvc *iam.IAM) error {
	if !o.requiresUserCreation() {
		valid, err := o.checkCredentials(iamSvc)
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("no accessKeyID %v found in the IAM user %v", o.iamUser.accessKeyID, o.iamUser.userName)
		}

		o.Printer.Verbosef("Using provided IAM credentials")
		return nil
	}

	o.Printer.Info.Printfln("Creating IAM user %v", o.iamUser.userName)
	createUserRequest := &iam.CreateUserInput{
		UserName: aws.String(o.iamUser.userName),
	}

	if _, err := iamSvc.CreateUser(createUserRequest); err != nil {
		if aerr, ok := err.(awserr.Error); ok { //nolint:errorlint // we need to access methods of the aws error interface
			switch aerr.Code() {
			case iam.ErrCodeEntityAlreadyExistsException:
				return fmt.Errorf("IAM user %v already exists, use --user-name flag to override it", o.iamUser.userName)
			default:
				return err
			}
		}
		return err
	}

	createAccessKeyRequest := &iam.CreateAccessKeyInput{
		UserName: aws.String(o.iamUser.userName),
	}

	o.Printer.Info.Printfln("Creating IAM access key for user %v", o.iamUser.userName)
	createAccessKeyResult, err := iamSvc.CreateAccessKey(createAccessKeyRequest)
	if err != nil {
		return err
	}

	o.iamUser.accessKeyID = *createAccessKeyResult.AccessKey.AccessKeyId
	o.iamUser.secretAccessKey = *createAccessKeyResult.AccessKey.SecretAccessKey

	if err = storeIamAccessKey(o.iamUser.userName,
		o.iamUser.accessKeyID,
		o.iamUser.secretAccessKey); err != nil {
		return err
	}

	return nil
}

func (o *Options) checkCredentials(iamSvc *iam.IAM) (bool, error) {
	listAccessKeysRequest := &iam.ListAccessKeysInput{
		UserName: aws.String(o.iamUser.userName),
	}

	listAccessKeysResult, err := iamSvc.ListAccessKeys(listAccessKeysRequest)
	if err != nil {
		return false, err
	}

	for i := range listAccessKeysResult.AccessKeyMetadata {
		accessKey := listAccessKeysResult.AccessKeyMetadata[i]
		if *accessKey.AccessKeyId == o.iamUser.accessKeyID {
			return true, nil
		}
	}

	return false, nil
}

func (o *Options) ensurePolicy(iamSvc *iam.IAM) (string, error) {
	policyDocument, err := getPolicyDocument()
	if err != nil {
		return "", err
	}

	createPolicyRequest := &iam.CreatePolicyInput{
		PolicyName:     aws.String(o.iamUser.policyName),
		PolicyDocument: aws.String(policyDocument),
	}

	createPolicyResult, err := iamSvc.CreatePolicy(createPolicyRequest)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok { //nolint:errorlint // we need to access methods of the aws error interface
			switch aerr.Code() {
			case iam.ErrCodeEntityAlreadyExistsException:
				return o.checkPolicy(iamSvc)
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

func (o *Options) getPolicyArn(iamSvc *iam.IAM) (string, error) {
	getUserResult, err := iamSvc.GetUser(&iam.GetUserInput{
		UserName: aws.String(o.iamUser.userName),
	})
	if err != nil {
		return "", err
	}

	splits := strings.Split(*getUserResult.User.Arn, ":")
	accountID := splits[4]

	return fmt.Sprintf("arn:aws:iam::%v:policy/%v", accountID, o.iamUser.policyName), nil
}

// checkPolicy checks that the retrieved policy has the required permission.
func (o *Options) checkPolicy(iamSvc *iam.IAM) (string, error) {
	arn, err := o.getPolicyArn(iamSvc)
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
		return "", fmt.Errorf("the %v IAM policy has not the permission required by Liqo, try deleting or updating it",
			o.iamUser.policyName)
	}

	return arn, nil
}
