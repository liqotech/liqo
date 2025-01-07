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

import "encoding/json"

// PolicyDocument is our definition of our policies to be uploaded to IAM.
type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
}

// StatementEntry will dictate what this policy will allow or not allow.
type StatementEntry struct {
	Effect   string
	Action   []string
	Resource string
}

var policy = PolicyDocument{
	Version: "2012-10-17",
	Statement: []StatementEntry{
		{
			Effect: "Allow",
			Action: []string{
				"iam:CreateUser",
				"iam:CreateAccessKey",
				"iam:ListAccessKeys",
				"iam:DeleteAccessKey",
			},
			Resource: "*",
		},
		{
			Effect: "Allow",
			Action: []string{
				"iam:GetUser",
				"iam:TagUser",
			},
			Resource: "arn:aws:iam::*:user/liqo-*",
		},
		{
			Effect: "Allow",
			Action: []string{
				"eks:DescribeCluster",
			},
			Resource: "*",
		},
	},
}

// getPolicyDocument returns the default policy document for the Liqo IAM user.
func getPolicyDocument() (string, error) {
	policyBytes, err := json.Marshal(policy)
	if err != nil {
		return "", err
	}
	return string(policyBytes), nil
}
