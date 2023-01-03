// Copyright 2019-2023 The Liqo Authors
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

package util

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

// RetrieveLiqoControllerManagerDeploymentArgs retrieves the list of arguments associated with the liqo controller manager deployment.
func RetrieveLiqoControllerManagerDeploymentArgs(ctx context.Context, cl client.Client, namespace string) ([]string, error) {
	// Retrieve the deployment of the liqo controller manager component
	var deployments appsv1.DeploymentList
	if err := cl.List(ctx, &deployments, client.InNamespace(namespace), client.MatchingLabelsSelector{
		Selector: liqolabels.ControllerManagerLabelSelector(),
	}); err != nil || len(deployments.Items) != 1 {
		return nil, errors.New("failed to retrieve the liqo controller manager deployment")
	}

	containers := deployments.Items[0].Spec.Template.Spec.Containers
	if len(containers) != 1 {
		return nil, errors.New("retrieved an invalid liqo controller manager deployment")
	}

	return containers[0].Args, nil
}

// RetrieveLiqoAuthDeploymentArgs retrieves the list of arguments associated with the liqo auth deployment.
func RetrieveLiqoAuthDeploymentArgs(ctx context.Context, cl client.Client, namespace string) ([]string, error) {
	// Retrieve the deployment of the liqo controller manager component
	selector, err := metav1.LabelSelectorAsSelector(&liqolabels.AuthServiceLabelSelector)
	if err != nil {
		return nil, errors.New("failed to forge the liqo auth deployment selector")
	}
	var deployments appsv1.DeploymentList
	if err := cl.List(ctx, &deployments, client.InNamespace(namespace), client.MatchingLabelsSelector{
		Selector: selector,
	}); err != nil || len(deployments.Items) != 1 {
		return nil, errors.New("failed to retrieve the liqo auth deployment")
	}

	containers := deployments.Items[0].Spec.Template.Spec.Containers
	if len(containers) != 1 {
		return nil, errors.New("retrieved an invalid liqo auth deployment")
	}

	return containers[0].Args, nil
}

// ExtractValueFromArgumentList extracts the argument value from an argument list.
func ExtractValueFromArgumentList(key string, argumentList []string) (string, error) {
	prefix := key + "="
	for _, argument := range argumentList {
		if strings.HasPrefix(argument, prefix) {
			return strings.Join(strings.Split(argument, "=")[1:], "="), nil
		}
	}
	return "", fmt.Errorf("argument not found")
}

// ExtractValuesFromArgumentListOrDefault extracts the argument value from an argument list or returns a default value.
func ExtractValuesFromArgumentListOrDefault(key string, argumentList []string, defaultValue string) string {
	value, err := ExtractValueFromArgumentList(key, argumentList)
	if err != nil {
		return defaultValue
	}
	return value
}

// ExtractValuesFromNestedMaps takes a map and a list of keys and visits a tree of nested maps
// using the keys in the order provided. At each iteration, if the number of non-visited keys
// is 1, the function returns the value associated to the last key, else if it is greater
// than 1, the function expects the value to be a map and a new recursive iteration happens.
// In case the key is not found, an empty string is returned.
// In case no keys are provided, an error is returned.
// Example:
//
//	m := map[string]interface{}{
//		"first": map[string]interface{}{
//			"second": map[string]interface{}{
//				"third": "value",
//			},
//		},
//	}
//	ValueFor(m, "first", "second", "third") // returns "value", nil
//	ValueFor(m, "first", "second") // returns map[string]interface{}{ "third": "value" }, nil
//	ValueFor(m, "first", "third") // returns "", nil
//	ValueFor(m) // returns nil, "At least one key is required"
func ExtractValuesFromNestedMaps(m map[string]interface{}, keys ...string) (val interface{}, err error) {
	var ok bool
	if len(keys) == 0 {
		return nil, fmt.Errorf("at least one key is required")
	} else if val, ok = m[keys[0]]; !ok {
		return "", nil
	} else if len(keys) == 1 {
		return val, nil
	} else if m, ok = val.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("the value for key %s is not map (expected to be a map)", keys[0])
	} else {
		return ExtractValuesFromNestedMaps(m, keys[1:]...)
	}
}

// ParseArgsMultipleValues parse a string containing multiple key/value couples separated by ',' (eg. key1=value1,key2=value2).
func ParseArgsMultipleValues(values, separator string) (map[string]string, error) {
	result := make(map[string]string)
	for _, value := range strings.Split(values, separator) {
		parts := strings.Split(value, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid value %s", value)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}
