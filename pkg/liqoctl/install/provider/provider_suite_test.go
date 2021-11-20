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
	flags.String("reserved-subnets", "", "")
	return flags
}

type commandTestcase struct {
	flags                     map[string]string
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
		Expect(provider.ValidateGenericCommandArguments(flags)).To(t.expectedValidationOutcome)
		Expect(provider.ClusterName).To(t.expectedClusterName)
	},
	Entry("should accept a valid name", commandTestcase{
		flags:                     map[string]string{"cluster-name": "test-cluster123"},
		expectedValidationOutcome: Succeed(),
		expectedClusterName:       Equal("test-cluster123"),
	}),
	Entry("should not accept an invalid name", commandTestcase{
		flags:                     map[string]string{"cluster-name": "Invalid cluster!"},
		expectedValidationOutcome: Not(Succeed()),
		expectedClusterName:       BeEmpty(),
	}),
	Entry("should generate a valid name if none is given", commandTestcase{
		flags:                     map[string]string{},
		expectedValidationOutcome: Succeed(),
		expectedClusterName:       Not(BeEmpty()),
	}),
)
