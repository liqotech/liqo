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

package args

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/klog/v2"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

// ClusterIDFlags stores the values of flags representing a ClusterID.
type ClusterIDFlags struct {
	local     bool
	ClusterID *string
}

var _ flag.Value = &ClusterIDFlags{}

// NewClusterIDFlags returns a set of command line flags to read a cluster identity.
// If local=true the identity refers to the local cluster, otherwise it refers to a foreign cluster.
// Set flags=nil to use command-line flags (os.Argv).
//
// Example usage:
//
//	fcFlags := NewClusterIDFlags(false, nil)
//	flag.Parse()
//	foreignClusterID := fcFlags.Read()
func NewClusterIDFlags(local bool, flags *pflag.FlagSet) ClusterIDFlags {
	var prefix, description string
	if local {
		prefix = "cluster" //nolint:goconst // No need to make the word "cluster" a const...
		description = "The %s of the current cluster"
	} else {
		prefix = "foreign-cluster"
		description = "The %s of the foreign cluster"
	}
	if flags == nil {
		flags = pflag.CommandLine
	}
	return ClusterIDFlags{
		local:     local,
		ClusterID: flags.String(fmt.Sprintf("%s-id", prefix), "", fmt.Sprintf(description, "ID")),
	}
}

// Read performs validation on the values passed and returns a ClusterID if successful.
func (f ClusterIDFlags) Read() (liqov1beta1.ClusterID, error) {
	var clusterWord string
	if f.local {
		clusterWord = "cluster"
	} else {
		clusterWord = "foreign cluster"
	}

	if *f.ClusterID == "" {
		return "", fmt.Errorf("the %s ID may not be empty", clusterWord)
	}
	errs := validation.IsDNS1123Label(*f.ClusterID)
	if len(errs) != 0 {
		return "",
			fmt.Errorf("the %s ID may only contain lowercase letters, numbers and hyphens, and must not be no longer than 63 characters", clusterWord)
	}

	return liqov1beta1.ClusterID(*f.ClusterID), nil
}

// ReadOrDie returns a ClusterID. It prints an error message and exits if the values are not valid.
func (f ClusterIDFlags) ReadOrDie() liqov1beta1.ClusterID {
	identity, err := f.Read()
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	return identity
}

// String implements the flag.Value interface.
func (f ClusterIDFlags) String() string {
	if f.ClusterID == nil {
		return ""
	}
	return *f.ClusterID
}

// Set implements the flag.Value interface.
func (f *ClusterIDFlags) Set(value string) error {
	f.ClusterID = &value
	return nil
}

// Type implements the flag.Value interface.
func (f ClusterIDFlags) Type() string {
	return "clusterID"
}

// GetClusterID returns the ClusterID stored in the flags.
func (f ClusterIDFlags) GetClusterID() liqov1beta1.ClusterID {
	if f.ClusterID == nil {
		return ""
	}
	return liqov1beta1.ClusterID(*f.ClusterID)
}
