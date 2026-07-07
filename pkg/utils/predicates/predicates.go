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

// Package predicates contains utility methods to create predicates.
package predicates

import (
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NewAnyLabelsSetPredicate returns a predicate that filters objects based on a list of label sets.
// The predicate will return true if the object matches at least one of the label sets.
func NewAnyLabelsSetPredicate(labelsSets []labels.Set) predicate.Predicate {
	return NewTypedAnyLabelsSetPredicate[client.Object](labelsSets)
}

// NewTypedAnyLabelsSetPredicate returns a typed predicate that filters objects based on a list of label sets.
// The predicate will return true if the object matches at least one of the label sets.
func NewTypedAnyLabelsSetPredicate[T client.Object](labelsSets []labels.Set) predicate.TypedPredicate[T] {
	labelPredicates := make([]predicate.TypedPredicate[T], len(labelsSets))
	for i := range labelsSets {
		labelPredicates[i] = predicate.NewTypedPredicateFuncs(func(obj T) bool {
			selector := labels.SelectorFromValidatedSet(labelsSets[i])
			return selector.Matches(labels.Set(obj.GetLabels()))
		})
	}
	return predicate.Or(labelPredicates...)
}

// NewAnyNamespacePredicate returns a predicate that filters objects based on a list of namespaces.
// The predicate will return true if the object is in at least one of the namespaces.
func NewAnyNamespacePredicate(namespaces []string) predicate.Predicate {
	return NewTypedAnyNamespacePredicate[client.Object](namespaces)
}

// NewTypedAnyNamespacePredicate returns a typed predicate that filters objects based on a list of namespaces.
// The predicate will return true if the object is in at least one of the namespaces.
func NewTypedAnyNamespacePredicate[T client.Object](namespaces []string) predicate.TypedPredicate[T] {
	namespacePredicates := make([]predicate.TypedPredicate[T], len(namespaces))
	for i := range namespaces {
		namespacePredicates[i] = predicate.NewTypedPredicateFuncs(func(obj T) bool {
			return obj.GetNamespace() == namespaces[i]
		})
	}
	return predicate.Or(namespacePredicates...)
}
