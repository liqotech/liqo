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

package provider_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"

	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
)

func TestProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider Suite")
}

func newFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("cluster-name", "", "")
	flags.Bool("generate-name", false, "")
	flags.String("reserved-subnets", "", "")
	flags.String("cluster-labels", "", "")
	return flags
}

type commandTestcase struct {
	flags                     map[string]string
	oldClusterName            string
	expectedValidationOutcome OmegaMatcher
	expectedClusterName       OmegaMatcher
}

var _ = DescribeTable("ValidateGenericCommandArguments",
	func(t commandTestcase) {
		flags := newFlagSet()
		for flag, value := range t.flags {
			Expect(flags.Set(flag, value)).To(Succeed())
		}
		provider := provider.GenericProvider{}
		Expect(provider.PreValidateGenericCommandArguments(flags)).To(Succeed())
		Expect(provider.PostValidateGenericCommandArguments(t.oldClusterName)).To(t.expectedValidationOutcome)
		Expect(provider.ClusterName).To(t.expectedClusterName)
	},
	Entry("should accept a valid name", commandTestcase{
		flags:                     map[string]string{"cluster-name": "test-cluster123"},
		oldClusterName:            "",
		expectedValidationOutcome: Succeed(),
		expectedClusterName:       Equal("test-cluster123"),
	}),
	Entry("should not accept an invalid name", commandTestcase{
		flags:                     map[string]string{"cluster-name": "Invalid cluster!"},
		oldClusterName:            "",
		expectedValidationOutcome: Not(Succeed()),
		expectedClusterName:       BeEmpty(),
	}),
	Entry("should generate a valid name if --generate-name is set", commandTestcase{
		flags:                     map[string]string{"generate-name": "true"},
		oldClusterName:            "",
		expectedValidationOutcome: Succeed(),
		expectedClusterName:       Not(BeEmpty()),
	}),
	Entry("should reuse the old cluster name", commandTestcase{
		flags:                     map[string]string{},
		oldClusterName:            "old-cluster-name",
		expectedValidationOutcome: Succeed(),
		expectedClusterName:       Equal("old-cluster-name"),
	}),
	Entry("should reuse the old cluster name if --generate-name is set", commandTestcase{
		flags:                     map[string]string{"generate-name": "true"},
		oldClusterName:            "old-cluster-name",
		expectedValidationOutcome: Succeed(),
		expectedClusterName:       Equal("old-cluster-name"),
	}),
	Entry("should set the new name if provided", commandTestcase{
		flags:                     map[string]string{"cluster-name": "test-cluster123"},
		oldClusterName:            "old-cluster-name",
		expectedValidationOutcome: Succeed(),
		expectedClusterName:       Equal("test-cluster123"),
	}),
	Entry("should not accept both cluster name and generate flags", commandTestcase{
		flags:                     map[string]string{"cluster-name": "test-cluster123", "generate-name": "true"},
		oldClusterName:            "",
		expectedValidationOutcome: Not(Succeed()),
		expectedClusterName:       BeEmpty(),
	}),
	Entry("the cluster name should be provided in some way", commandTestcase{
		flags:                     map[string]string{},
		oldClusterName:            "",
		expectedValidationOutcome: Not(Succeed()),
		expectedClusterName:       BeEmpty(),
	}),
)
