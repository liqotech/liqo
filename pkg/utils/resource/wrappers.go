// Copyright 2019-2026 The Liqo Authors
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

package resource

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var knownVariables = []string{"namespace", "name", "kind", "group"}

// CreateOrUpdate is a wrapper around controllerutil.CreateOrUpdate that automatically adds global labels and annotations.
// It takes the same parameters as controllerutil.CreateOrUpdate plus an optional list of additional labels to merge.
func CreateOrUpdate(ctx context.Context, c client.Client, obj client.Object,
	mutateFn func() error) (controllerutil.OperationResult, error) {
	// Create a wrapper mutation function that adds labels before calling the original
	wrappedMutateFn := func() error {
		// First call the original mutation function
		if err := mutateFn(); err != nil {
			return err
		}

		// Build the variables map from the object to replace variables in global labels and annotations.
		variablesMap, err := buildVariablesMap(obj, c.Scheme())
		if err != nil {
			return fmt.Errorf("unable replace variable in global labels and annotations: %w", err)
		}

		// Then add global labels and any additional labels
		if obj.GetLabels() == nil {
			obj.SetLabels(make(map[string]string))
		}
		labels := obj.GetLabels()
		for k, v := range GetGlobalLabels() {
			labels[k] = replaceVariables(v, variablesMap)
		}
		obj.SetLabels(labels)

		// Add global annotations
		if obj.GetAnnotations() == nil {
			obj.SetAnnotations(make(map[string]string))
		}
		annotations := obj.GetAnnotations()
		for k, v := range GetGlobalAnnotations() {
			annotations[k] = replaceVariables(v, variablesMap)
		}
		obj.SetAnnotations(annotations)

		return nil
	}

	return controllerutil.CreateOrUpdate(ctx, c, obj, wrappedMutateFn)
}

func getVar(variable string, obj client.Object, kind schema.GroupVersionKind) string {
	switch variable {
	case "namespace":
		return obj.GetNamespace()
	case "name":
		return obj.GetName()
	case "kind":
		return kind.Kind
	case "group":
		return kind.Group
	}

	return ""
}

func buildVariablesMap(obj client.Object, scheme *runtime.Scheme) (map[string]string, error) {
	variables := make(map[string]string)
	groupKind, _, err := scheme.ObjectKinds(obj)

	if err != nil {
		return nil, fmt.Errorf("unable to retrieve object kind: %w", err)
	}

	for _, variable := range knownVariables {
		variables[variable] = getVar(variable, obj, groupKind[0])
	}
	return variables, nil
}

// replaceVariables replace the variables in the given string with the corresponding values in the object.
func replaceVariables(s string, variablesMap map[string]string) string {
	// Replace the variables in the string with the corresponding values in the object
	for varName, varValue := range variablesMap {
		s = strings.ReplaceAll(s, "${"+varName+"}", varValue)
	}

	return s
}
