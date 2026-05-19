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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
)

// IsConfigurationObserved reports whether the Configuration controller has fully reconciled
// the current spec generation. Downstream consumers should gate their work on this check so
// they never act on a Configuration whose status reflects an older spec.
//
// The metadata.generation == status.observedGeneration check is the canonical Kubernetes
// freshness signal. The additional length and AllNonVoid checks are belt-and-suspenders: if
// the producing controller writes both fields correctly, they follow from generation parity;
// they catch the case where status fields were written out of order or partially populated.
func IsConfigurationObserved(cfg *networkingv1beta1.Configuration) bool {
	if cfg.Status.Remote == nil {
		return false
	}
	if cfg.Status.ObservedGeneration != cfg.Generation {
		return false
	}
	if len(cfg.Status.Remote.CIDR.Pod) != len(cfg.Spec.Remote.CIDR.Pod) ||
		len(cfg.Status.Remote.CIDR.External) != len(cfg.Spec.Remote.CIDR.External) {
		return false
	}
	return cidrutils.AllNonVoid(cfg.Status.Remote.CIDR.Pod) &&
		cidrutils.AllNonVoid(cfg.Status.Remote.CIDR.External)
}

// IsConfigurationObservedPredicate returns a controller-runtime predicate that admits only
// Configurations for which the controller has fully reconciled the current spec generation.
// Wire it via builder.WithPredicates(...) on any controller that consumes the status arrays.
func IsConfigurationObservedPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		cfg, ok := obj.(*networkingv1beta1.Configuration)
		if !ok {
			return false
		}
		return IsConfigurationObserved(cfg)
	})
}
