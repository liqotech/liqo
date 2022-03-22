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

package autocompletion

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/common"
)

// GetNamespaceNames returns a list of namespaces that start with the given string.
func GetNamespaceNames(ctx context.Context, startWith string) ([]string, error) {
	config, err := common.GetLiqoctlRestConf()
	if err != nil {
		return nil, err
	}

	k8sClient, err := client.New(config, client.Options{})
	if err != nil {
		return nil, err
	}

	namespaces := &corev1.NamespaceList{}
	if err := k8sClient.List(ctx, namespaces); err != nil {
		return nil, err
	}

	var names []string
	for i := range namespaces.Items {
		ns := &namespaces.Items[i]
		if strings.HasPrefix(ns.Name, startWith) {
			names = append(names, ns.Name)
		}
	}

	return names, nil
}
