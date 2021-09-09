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
