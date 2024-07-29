// Copyright 2019-2024 The Liqo Authors
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

package configuration

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// IsConfigurationStatusSet check if a Configuration is ready by checking if its status is correctly set.
func IsConfigurationStatusSet(ctx context.Context, cl client.Client, name, namespace string) (bool, error) {
	conf := &networkingv1beta1.Configuration{}
	if err := cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, conf); err != nil {
		return false, err
	}

	return conf.Status.Remote != nil &&
			conf.Status.Remote.CIDR.Pod.String() != "" &&
			conf.Status.Remote.CIDR.External.String() != "",
		nil
}
