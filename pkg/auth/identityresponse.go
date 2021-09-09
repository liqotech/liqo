// Copyright 2019-2021 The Liqo Authors
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

package auth

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
	"github.com/liqotech/liqo/pkg/utils/apiserver"
)

// AWSIdentityInfo contains the information required by a cluster to get a valied IAM-based identity.
type AWSIdentityInfo struct {
	AccessKeyID     string `json:"accessKeyID"`
	SecretAccessKey string `json:"secretAccessKey"`
	Region          string `json:"region"`
	EKSClusterID    string `json:"eksClusterID"`
	IAMUserArn      string `json:"iamUserArn"`
}

// CertificateIdentityResponse is the response on a certificate identity request.
type CertificateIdentityResponse struct {
	Namespace    string `json:"namespace"`
	Certificate  string `json:"certificate,omitempty"`
	APIServerURL string `json:"apiServerUrl"`
	APIServerCA  string `json:"apiServerCA,omitempty"`

	AWSIdentityInfo AWSIdentityInfo `json:"aws,omitempty"`
}

// HasAWSValues checks if the response has all the required AWS fields set.
func (resp *CertificateIdentityResponse) HasAWSValues() bool {
	credentials := resp.AWSIdentityInfo.AccessKeyID != "" && resp.AWSIdentityInfo.SecretAccessKey != ""
	region := resp.AWSIdentityInfo.Region != ""
	cluster := resp.AWSIdentityInfo.EKSClusterID != ""
	userArn := resp.AWSIdentityInfo.IAMUserArn != ""
	return credentials && region && cluster && userArn
}

// NewCertificateIdentityResponse makes a new CertificateIdentityResponse.
func NewCertificateIdentityResponse(
	namespace string, identityResponse *responsetypes.SigningRequestResponse,
	apiServerConfig apiserver.Config,
	clientset kubernetes.Interface, restConfig *rest.Config) (*CertificateIdentityResponse, error) {
	responseType := identityResponse.ResponseType

	switch responseType {
	case responsetypes.SigningRequestResponseCertificate:
		apiServerURL, err := apiserver.GetURL(apiServerConfig, clientset)
		if err != nil {
			klog.Error(err)
			return nil, err
		}

		var apiServerCa string
		if apiServerConfig.TrustedCA {
			apiServerCa = ""
		} else {
			apiServerCa, err = getAPIServerCA(restConfig)
			if err != nil {
				klog.Error(err)
				return nil, err
			}
		}

		return &CertificateIdentityResponse{
			Namespace:    namespace,
			Certificate:  base64.StdEncoding.EncodeToString(identityResponse.Certificate),
			APIServerURL: apiServerURL,
			APIServerCA:  apiServerCa,
		}, nil

	case responsetypes.SigningRequestResponseIAM:
		return &CertificateIdentityResponse{
			Namespace:    namespace,
			APIServerURL: *identityResponse.AwsIdentityResponse.EksCluster.Endpoint,
			APIServerCA:  *identityResponse.AwsIdentityResponse.EksCluster.CertificateAuthority.Data,
			AWSIdentityInfo: AWSIdentityInfo{
				EKSClusterID:    *identityResponse.AwsIdentityResponse.EksCluster.Name,
				AccessKeyID:     *identityResponse.AwsIdentityResponse.AccessKey.AccessKeyId,
				SecretAccessKey: *identityResponse.AwsIdentityResponse.AccessKey.SecretAccessKey,
				Region:          identityResponse.AwsIdentityResponse.Region,
				IAMUserArn:      identityResponse.AwsIdentityResponse.IamUserArn,
			},
		}, nil

	default:
		err := fmt.Errorf("unknown response type %v", responseType)
		klog.Error(err)
		return nil, err
	}

}

// getAPIServerCA retrieves the ApiServerCA.
// It can take it from the CAData in the restConfig, or reading it from the CAFile.
func getAPIServerCA(restConfig *rest.Config) (string, error) {
	if restConfig.CAData != nil && len(restConfig.CAData) > 0 {
		// CAData available in the restConfig, encode and return it.
		return base64.StdEncoding.EncodeToString(restConfig.CAData), nil
	}
	if restConfig.CAFile != "" {
		// CAData is not available, read it from the CAFile.
		dat, err := ioutil.ReadFile(restConfig.CAFile)
		if err != nil {
			klog.Error(err)
			return "", err
		}
		return base64.StdEncoding.EncodeToString(dat), nil
	}
	klog.Warning("empty CA data")
	return "", nil
}
