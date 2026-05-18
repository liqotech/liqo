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

package utils

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// AreConfigurationNetworkCIDRsConfigured reports whether the Configuration controller has fully reconciled
// the current spec generation. Downstream consumers should gate their work on this check so
// they never act on a Configuration whose status reflects an older spec.
func AreConfigurationNetworkCIDRsConfigured(cfg *networkingv1beta1.Configuration) bool {
	cond := meta.FindStatusCondition(cfg.Status.Conditions, networkingv1beta1.ConfigurationConditionNetworkCIDRsConfigured)
	if cond == nil || cfg.Generation != cond.ObservedGeneration {
		return false
	}
	return cond.Status == metav1.ConditionTrue
}

// AreConfigurationNetworkCIDRsConfiguredPredicate returns a controller-runtime predicate that admits only
// Configurations for which the controller has fully reconciled the current spec generation.
// Wire it via builder.WithPredicates(...) on any controller that consumes the status arrays.
func AreConfigurationNetworkCIDRsConfiguredPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		cfg, ok := obj.(*networkingv1beta1.Configuration)
		if !ok {
			return false
		}
		return AreConfigurationNetworkCIDRsConfigured(cfg)
	})
}
