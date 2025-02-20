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

package utils

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

// GetCtrlManagerContainer retrieves the container of the controller manager from the deployment.
func GetCtrlManagerContainer(ctrlDeployment *appsv1.Deployment) (*corev1.Container, error) {
	// Get the container of the controller manager
	containers := ctrlDeployment.Spec.Template.Spec.Containers
	for i := range containers {
		if containers[i].Name == consts.ControllerManagerAppName {
			return &containers[i], nil
		}
	}

	return nil, fmt.Errorf("invalid controller manager deployment: no container with name %q found", consts.ControllerManagerAppName)
}

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

// ExtractValuesFromArgumentList extracts the argument value from an argument list.
// When the argument is found, ok is true and the value is returned.
// When the argument is found but no value is provided, ok is true and an empty string is returned.
// When the argument is not found, ok is false and the value is an empty string.
func ExtractValuesFromArgumentList(key string, argumentList []string) (values string, err error) {
	prefix := key + "="
	for _, argument := range argumentList {
		if strings.HasPrefix(argument, prefix) {
			return strings.Join(strings.Split(argument, "=")[1:], "="), nil
		} else if key == argument {
			return "", nil
		}
	}
	return "", fmt.Errorf("argument %s not found", key)
}

// ExtractValuesFromArgumentListOrDefault extracts the argument value from an argument list or returns a default value.
func ExtractValuesFromArgumentListOrDefault(key string, argumentList []string, defaultValue string) string {
	value, err := ExtractValuesFromArgumentList(key, argumentList)
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
