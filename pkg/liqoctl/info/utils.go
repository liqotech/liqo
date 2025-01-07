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
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// collectDataFromCheckers collect the data retrieved by the checkers in a map.
func (o *Options) collectDataFromCheckers(checkers []Checker) map[string]interface{} {
	data := map[string]interface{}{}

	for i := range checkers {
		data[checkers[i].GetID()] = checkers[i].GetData()
	}

	return data
}

// collectDataFromMultiClusterCheckers collect the data retrieved by the checkers in a map.
func (o *Options) collectDataFromMultiClusterCheckers(checkers []MultiClusterChecker) (map[liqov1beta1.ClusterID]map[string]interface{}, error) {
	data := map[liqov1beta1.ClusterID]map[string]interface{}{}
	for clusterID := range o.ClustersInfo {
		data[clusterID] = map[string]interface{}{}
		for _, c := range checkers {
			checkerID := c.GetID()
			res, err := c.GetDataByClusterID(clusterID)
			if err != nil {
				o.Printer.Warning.Printfln("%s: %v", checkerID, err)
				return nil, err
			}

			data[clusterID][checkerID] = res
		}
	}
	return data, nil
}

func (o *Options) getClusterFromQuery(inQuery string, clusterIDs []liqov1beta1.ClusterID) (newQuery, selectedCluster string) {
	// Get the cluster selected by the query
	newQuery = strings.TrimPrefix(inQuery, ".")
	splittedQuery := strings.Split(newQuery, ".")
	selectedCluster = splittedQuery[0]
	// If we have one single cluster, then allow the user to omit the cluster name
	firstCluster := string(clusterIDs[0])
	if len(clusterIDs) == 1 && selectedCluster != firstCluster {
		selectedCluster = firstCluster
	} else {
		newQuery = strings.Join(splittedQuery[1:], ".")
	}
	return
}

func (o *Options) getForeignClusters(ctx context.Context, clusterIDs []string) error {
	o.ClustersInfo = map[liqov1beta1.ClusterID]*liqov1beta1.ForeignCluster{}

	var foreignClusterList liqov1beta1.ForeignClusterList

	// Get all the clusters if no clusterID has been specified
	labelSelector := labels.NewSelector()
	if len(clusterIDs) > 0 {
		labelFilter, _ := labels.NewRequirement(consts.RemoteClusterID, selection.In, clusterIDs)
		labelSelector = labelSelector.Add(*labelFilter)
	}

	if err := o.CRClient.List(ctx, &foreignClusterList, &client.ListOptions{
		LabelSelector: labelSelector,
	}); err != nil {
		errmsg := fmt.Sprintf("Unable to retrieve peer clusters: %v", err)
		o.Printer.Error.Println(errmsg)
		return errors.New(errmsg)
	}

	// Collect the results
	for i := range foreignClusterList.Items {
		cluster := &foreignClusterList.Items[i]
		o.ClustersInfo[cluster.Spec.ClusterID] = cluster
	}

	if len(clusterIDs) > len(o.ClustersInfo) {
		// Check what clusters are not found
		var notfound []string
		for _, id := range clusterIDs {
			if _, ok := o.ClustersInfo[liqov1beta1.ClusterID(id)]; !ok {
				notfound = append(notfound, id)
			}
		}
		errmsg := fmt.Sprintf("Peer clusters %q not found in active peerings", strings.Join(notfound, ", "))
		o.Printer.Error.Println(errmsg)
		return errors.New(errmsg)
	}
	return nil
}

// installationCheck checks if Liqo is installed in the cluster.
func (o *Options) installationCheck(ctx context.Context) error {
	_, err := o.KubeClient.CoreV1().Namespaces().Get(ctx, o.LiqoNamespace, metav1.GetOptions{})

	switch {
	case client.IgnoreNotFound(err) != nil:
		o.Printer.Error.Printfln("Unable to check if Liqo is installed: %v", err)
		return err
	case kerrors.IsNotFound(err):
		o.Printer.Error.Println("Liqo is not installed in the current cluster! \n\n" +
			"You can install liqo via the 'liqoctl install' command.\n" +
			"Check 'liqoctl install --help' for further information.")
		return err
	}

	return nil
}

// sPrintField returns a specific field from the data collected from the checkers
// given a query in dot notation.
func (o *Options) sPrintField(query string, data map[string]interface{}, queryShortcuts map[string]string) (string, error) {
	// Check whether the query is actually a shortcut
	if shortcut, ok := queryShortcuts[strings.ToLower(query)]; ok {
		query = shortcut
	}

	query = strings.TrimPrefix(query, ".")
	fields := strings.Split(query, ".")

	currData, ok := data[fields[0]]
	if !ok {
		return "", fmt.Errorf("invalid query %q: %q not found", query, fields[0])
	}

	for i, f := range fields[1:] {
		if reflect.ValueOf(currData).Kind() != reflect.Struct {
			// We need to report that the previous field is not an object.
			// We use fields[i] as we iterate over `fields[1:]` so "i" is already pointing to the value before "f"
			return "", fmt.Errorf("invalid query %q: %q is not an object", query, fields[i])
		}

		gotData := reflect.ValueOf(currData).FieldByNameFunc(func(fieldName string) bool {
			return strings.EqualFold(fieldName, f)
		})

		if !gotData.IsValid() || (gotData.IsValid() && gotData.IsZero()) {
			return "", fmt.Errorf("invalid query %q: %q not found", query, f)
		}
		currData = gotData.Interface()
	}

	// Check the type of returned data to correctly print the output
	kind := reflect.ValueOf(currData).Kind()
	if kind != reflect.Struct && kind != reflect.Slice && kind != reflect.Map {
		return fmt.Sprint(currData), nil
	}

	return o.sPrintMachineReadable(currData)
}

// sPrintOutput format the data as the output format.
func (o *Options) sPrintMachineReadable(data interface{}) (string, error) {
	var output string
	if o.Format == JSON {
		jsonRes, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return "", err
		}
		output = string(jsonRes)
	} else {
		// if not json, print the output in yaml format
		yamlRes, err := yaml.Marshal(data)
		if err != nil {
			return "", err
		}
		output = string(yamlRes)
	}

	return output, nil
}
