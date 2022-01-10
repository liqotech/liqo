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

package args

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// ClusterIdentityFlags stores the values of flags representing a ClusterIdentity.
type ClusterIdentityFlags struct {
	local       bool
	ClusterID   *string
	ClusterName *string
}

// NewClusterIdentityFlags returns a set of command line flags to read a cluster identity.
// If local=true the identity refers to the local cluster, otherwise it refers to a foreign cluster.
// Set flags=nil to use command-line flags (os.Argv).
//
// Example usage:
//
//   fcFlags := NewClusterIdentityFlags(false, nil)
//   flag.Parse()
//   foreignClusterIdentity := fcFlags.Read()
func NewClusterIdentityFlags(local bool, flags *flag.FlagSet) ClusterIdentityFlags {
	var prefix, description string
	if local {
		prefix = "cluster" // nolint:goconst // No need to make the word "cluster" a const...
		description = "The %s of the current cluster"
	} else {
		prefix = "foreign-cluster"
		description = "The %s of the foreign cluster"
	}
	if flags == nil {
		flags = flag.CommandLine
	}
	return ClusterIdentityFlags{
		local:       local,
		ClusterID:   flags.String(fmt.Sprintf("%s-id", prefix), "", fmt.Sprintf(description, "ID")),
		ClusterName: flags.String(fmt.Sprintf("%s-name", prefix), "", fmt.Sprintf(description, "name")),
	}
}

// Read performs validation on the values passed and returns a ClusterIdentity if successful.
func (f ClusterIdentityFlags) Read() (discoveryv1alpha1.ClusterIdentity, error) {
	var clusterWord string
	if f.local {
		clusterWord = "cluster"
	} else {
		clusterWord = "foreign cluster"
	}

	if *f.ClusterID == "" {
		return discoveryv1alpha1.ClusterIdentity{}, fmt.Errorf("the %s ID may not be empty", clusterWord)
	}
	if *f.ClusterName == "" {
		return discoveryv1alpha1.ClusterIdentity{}, fmt.Errorf("the %s name may not be empty", clusterWord)
	}
	errs := validation.IsDNS1123Label(*f.ClusterID)
	if len(errs) != 0 {
		return discoveryv1alpha1.ClusterIdentity{},
			fmt.Errorf("the %s ID may only contain lowercase letters, numbers and hyphens, and must not be no longer than 63 characters", clusterWord)
	}
	errs = validation.IsDNS1123Label(*f.ClusterName)
	if len(errs) != 0 {
		return discoveryv1alpha1.ClusterIdentity{},
			fmt.Errorf("the %s name may only contain lowercase letters, numbers and hyphens, and must not be no longer than 63 characters", clusterWord)
	}

	return discoveryv1alpha1.ClusterIdentity{
		ClusterID:   *f.ClusterID,
		ClusterName: *f.ClusterName,
	}, nil
}

// ReadOrDie returns a ClusterIdentity. It prints an error message and exits if the values are not valid.
func (f ClusterIdentityFlags) ReadOrDie() discoveryv1alpha1.ClusterIdentity {
	identity, err := f.Read()
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	return identity
}
