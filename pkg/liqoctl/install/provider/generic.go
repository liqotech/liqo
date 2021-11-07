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
	"fmt"
	"math/rand"
	"strings"

	"github.com/goombaio/namegenerator"
	flag "github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/validation"

	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// Providers list of providers supported by liqoctl.
var Providers = []string{"kubeadm", "kind", "k3s", "eks", "gke", "aks", "openshift"}

// GenericProvider includes the fields and the logic required by every install provider.
type GenericProvider struct {
	ReservedSubnets []string
	ClusterLabels   map[string]string
	ClusterName     string
}

// ValidateGenericCommandArguments validates the flags required by every install provider.
func (p *GenericProvider) ValidateGenericCommandArguments(flags *flag.FlagSet) (err error) {
	generate, err := flags.GetBool("generate-name")
	if err != nil {
		return err
	}
	clusterName, err := flags.GetString("cluster-name")
	if err != nil {
		return err
	}

	if clusterName == "" && !generate {
		return fmt.Errorf("you must provide a cluster name or use --generate-name")
	}
	if clusterName != "" && generate {
		fmt.Printf("%#v %#v\n", clusterName, generate)
		return fmt.Errorf("cannot set a cluster name and use --generate-name at the same time")
	}
	if generate {
		randomName := namegenerator.NewNameGenerator(rand.Int63()).Generate() // nolint:gosec // don't need crypto/rand
		randomName = strings.Replace(randomName, "_", "-", 1)
		clusterName = randomName
		fmt.Printf("A random cluster name was generated for you: %s\n", clusterName)
	}
	errs := validation.IsDNS1123Label(clusterName)
	if len(errs) != 0 {
		return fmt.Errorf("the cluster name may only contain lowercase letters, numbers and hyphens, and must not be no longer than 63 characters")
	}
	p.ClusterName = clusterName

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
