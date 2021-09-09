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

package provider

import (
	flag "github.com/spf13/pflag"

	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// GenericProvider includes the fields and the logic required by every install provider.
type GenericProvider struct {
	ReservedSubnets []string
	ClusterLabels   map[string]string
	ClusterName     string
}

// ValidateGenericCommandArguments validates the flags required by every install provider.
func (p *GenericProvider) ValidateGenericCommandArguments(flags *flag.FlagSet) (err error) {
	p.ClusterName, err = flags.GetString("cluster-name")
	if err != nil {
		return err
	}

	subnetString, err := flags.GetString("reserved-subnets")
	if err != nil {
		return err
	}

	reservedSubnets := argsutils.CIDRList{}
	if err = reservedSubnets.Set(subnetString); err != nil {
		return err
	}

	p.ReservedSubnets = reservedSubnets.StringList.StringList

	return nil
}
