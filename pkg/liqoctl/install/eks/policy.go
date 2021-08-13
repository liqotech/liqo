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
			},
			Resource: "*",
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
