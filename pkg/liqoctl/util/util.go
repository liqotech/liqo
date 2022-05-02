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

package util

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqocontrollermanager "github.com/liqotech/liqo/pkg/liqo-controller-manager"
)

// RetrieveLiqoControllerManagerDeploymentArgs retrieves the list of arguments associated with the liqo controller manager deployment.
func RetrieveLiqoControllerManagerDeploymentArgs(ctx context.Context, cl client.Client, namespace string) ([]string, error) {
	// Retrieve the deployment of the liqo controller manager component
	var deployments appsv1.DeploymentList
	if err := cl.List(ctx, &deployments, client.InNamespace(namespace), client.MatchingLabelsSelector{
		Selector: liqocontrollermanager.DeploymentLabelSelector(),
	}); err != nil || len(deployments.Items) != 1 {
		return nil, errors.New("failed to retrieve the liqo controller manager deployment")
	}

	containers := deployments.Items[0].Spec.Template.Spec.Containers
	if len(containers) != 1 {
		return nil, errors.New("retrieved an invalid liqo controller manager deployment")
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
