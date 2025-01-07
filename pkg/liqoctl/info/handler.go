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
//

package info

import (
	"context"
	"fmt"
	"maps"
	"slices"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
)

// OutputFormat represents the format of the output of the command.
type OutputFormat string

const (
	// JSON indicates that the output will be in JSON format.
	JSON OutputFormat = "json"
	// YAML indicates that the output will be in YAML format.
	YAML OutputFormat = "yaml"
)

// LocalInfoQueryShortcuts contains shortcuts for the paths in the local info data.
var LocalInfoQueryShortcuts = map[string]string{
	"clusterid": "local.clusterid",
}

// Options encapsulates the arguments of the info command.
type Options struct {
	*factory.Factory

	Verbose      bool
	Format       OutputFormat
	GetQuery     string
	ClustersInfo map[liqov1beta1.ClusterID]*liqov1beta1.ForeignCluster
}

// NewOptions returns a new Options struct.
func NewOptions(f *factory.Factory) *Options {
	return &Options{
		Factory: f,
	}
}

// RunInfo execute the `info` command.
func (o *Options) RunInfo(ctx context.Context, checkers []Checker) error {
	// Check whether Liqo is installed in the current cluster
	if err := o.installationCheck(ctx); err != nil {
		return err
	}

	// Start collecting the data via the checkers
	for i := range checkers {
		checkers[i].Collect(ctx, *o)
		for _, err := range checkers[i].GetCollectionErrors() {
			o.Printer.Warning.Println(err)
		}
	}

	var err error
	var output string
	switch {
	// If no format is specified, format and print a user-friendly output
	case o.Format == "" && o.GetQuery == "":
		for i := range checkers {
			o.Printer.BoxSetTitle(checkers[i].GetTitle())
			o.Printer.BoxPrintln(checkers[i].Format(*o))
		}
		return nil
	// If query specified try to retrieve the field from the output
	case o.GetQuery != "":
		data := o.collectDataFromCheckers(checkers)
		output, err = o.sPrintField(o.GetQuery, data, LocalInfoQueryShortcuts)
	default:
		data := o.collectDataFromCheckers(checkers)
		output, err = o.sPrintMachineReadable(data)
	}

	if err != nil {
		o.Printer.Error.Println(err)
	} else {
		fmt.Println(output)
	}

	return err
}

// RunPeerInfo execute the `info peer` command.
func (o *Options) RunPeerInfo(ctx context.Context, checkers []MultiClusterChecker, clusterIDs []string) error {
	// Check whether Liqo is installed in the current cluster
	if err := o.installationCheck(ctx); err != nil {
		return err
	}

	if err := o.getForeignClusters(ctx, clusterIDs); err != nil {
		return err
	}

	// Start collecting the data via the checkers
	for i := range checkers {
		checkers[i].Collect(ctx, *o)
		for _, err := range checkers[i].GetCollectionErrors() {
			o.Printer.Warning.Println(err)
		}
	}

	var err error
	var output string
	switch {
	// If no format is specified, format and print a user-friendly output
	case o.Format == "" && o.GetQuery == "":
		clustersCounter := 0
		nPeers := len(o.ClustersInfo)
		for clusterID := range o.ClustersInfo {
			for i := range checkers {
				o.Printer.BoxSetTitle(checkers[i].GetTitle())
				o.Printer.BoxPrintln(checkers[i].FormatForClusterID(clusterID, *o))
			}

			clustersCounter++
			if clustersCounter < nPeers {
				fmt.Printf("\n\n")
			}
		}
		return nil
	// If query specified try to retrieve the field from the output
	case o.GetQuery != "":
		data, _ := o.collectDataFromMultiClusterCheckers(checkers)

		selectedClusterIDs := slices.Collect(maps.Keys(o.ClustersInfo))
		// Get the cluster selected by the query
		query, selectedCluster := o.getClusterFromQuery(o.GetQuery, selectedClusterIDs)

		// Get the field from the cluster data
		if showData, ok := data[liqov1beta1.ClusterID(selectedCluster)]; ok {
			output, err = o.sPrintField(query, showData, nil)
		} else {
			err = fmt.Errorf(
				"cluster %q in query %q is not among the requested clusters",
				selectedCluster,
				o.GetQuery,
			)
		}
	default:
		data, _ := o.collectDataFromMultiClusterCheckers(checkers)
		output, err = o.sPrintMachineReadable(data)
	}

	if err != nil {
		o.Printer.Error.Println(err)
	} else {
		fmt.Println(output)
	}

	return err
}
