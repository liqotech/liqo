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

package maps

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/liqotech/liqo/pkg/consts"
)

// Merge merges two maps.
func Merge[K comparable, V any](m1, m2 map[K]V) map[K]V {
	if m1 == nil {
		return m2
	}
	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}

// Sub removes elements of m2 from m1.
func Sub[K comparable, V any](m1, m2 map[K]V) map[K]V {
	for k := range m2 {
		delete(m1, k)
	}
	return m1
}

// FilterType is a function type used to filter a map.
type FilterType[K comparable] func(key K) bool

// Filter filters a map, returning a duplicate which contains only the elements matching the filter function.
func Filter[K comparable, V any](m map[K]V, filter FilterType[K]) map[K]V {
	filtered := make(map[K]V)

	for k, v := range m {
		if filter(k) {
			filtered[k] = v
		}
	}

	return filtered
}

// FilterWhitelist returns a filter function returning true if the key is in the whitelist.
func FilterWhitelist[K comparable](whitelist ...K) FilterType[K] {
	return func(check K) bool {
		for _, el := range whitelist {
			if el == check {
				return true
			}
		}
		return false
	}
}

// FilterBlacklist returns a filter function returning true if the key is not the blacklist.
func FilterBlacklist[K comparable](blacklist ...K) FilterType[K] {
	return func(check K) bool {
		return !FilterWhitelist(blacklist...)(check)
	}
}

// SmartMergeLabels merges labels from a template map in a map, and remember what labels were added in
// the object from the template, storing them in a custom annotation. This allows the function to also
// delete the labels that were added by the template previously, but that they are no longer present
// in the template. This is useful to avoid to accumulate labels in the object that are not present
// in the template anymore.
func SmartMergeLabels(obj metav1.Object, templateLabels map[string]string) {
	if templateLabels == nil {
		templateLabels = make(map[string]string)
	}

	// Filter out the labels not present in the template but present in the cached template labels
	filteredLabels := FilteredDeletedLabels(obj.GetAnnotations(), obj.GetLabels(), templateLabels)

	// Merge with current template labels
	obj.SetLabels(labels.Merge(filteredLabels, templateLabels))

	// Update cache with latest template labels
	obj.SetAnnotations(UpdateCache(obj.GetAnnotations(), templateLabels, consts.LabelsTemplateAnnotationKey))
}

// SmartMergeAnnotations merges annotations from a template map in a map, and remember what annotations were added in
// the object from the template, storing them in a custom annotation. This allows the function to also
// delete the annotations that were added by the template previously, but that they are no longer present
// in the template. This is useful to avoid to accumulate annotations in the object that are not present
// in the template anymore.
func SmartMergeAnnotations(obj metav1.Object, templateAnnots map[string]string) {
	if templateAnnots == nil {
		templateAnnots = make(map[string]string)
	}

	// Filter out the annotations not present in the template but present in the cached template annotations
	filteredAnnots := FilteredDeletedAnnotations(obj.GetAnnotations(), templateAnnots)

	// Merge with current template annotations
	obj.SetAnnotations(labels.Merge(filteredAnnots, templateAnnots))

	// Update cache with latest template annotations
	obj.SetAnnotations(UpdateCache(obj.GetAnnotations(), templateAnnots, consts.AnnotsTemplateAnnotationKey))
}

// FilteredDeletedLabels returns the labels of the object after deleting the labels not present
// in the template but present in the cached template labels (e.g., the ones added by the template).
func FilteredDeletedLabels(annots, labs, templateLabs map[string]string) map[string]string {
	// Get cached labels of the template
	var cachedTemplateLabelsKey []string
	if annots != nil {
		cache := annots[consts.LabelsTemplateAnnotationKey]
		cachedTemplateLabelsKey = DeSerializeCache(cache)
	}

	// Delete from objet labels the entries not present in the template labels
	return FilteredDeletedEntries(labs, templateLabs, cachedTemplateLabelsKey)
}

// FilteredDeletedAnnotations returns the annotations of the object after deleting the annotations not present
// in the template but present in the cached template annotations (e.g., the ones added by the template).
func FilteredDeletedAnnotations(annots, templateAnnots map[string]string) map[string]string {
	// Get cached annotations of the template
	var cachedTemplateAnnotsKeys []string
	if annots != nil {
		cache := annots[consts.AnnotsTemplateAnnotationKey]
		cachedTemplateAnnotsKeys = DeSerializeCache(cache)
	}

	// Delete from objet annotations the entries not present in the template annotations
	return FilteredDeletedEntries(annots, templateAnnots, cachedTemplateAnnotsKeys)
}

// FilteredDeletedEntries deletes entries of map m1 that are not present in map m2,
// excluding the ones not stored in a cache of keys.
func FilteredDeletedEntries(m1, m2 map[string]string, cache []string) map[string]string {
	res := maps.Clone(m1)
	if res == nil {
		res = make(map[string]string)
	}
	if m2 == nil {
		m2 = make(map[string]string)
	}
	for _, key := range cache {
		if _, exists := m2[key]; !exists {
			delete(res, key)
		}
	}
	return res
}

// UpdateCache updates the cache with the latest entries from template.
func UpdateCache(annots, template map[string]string, cacheKey string) map[string]string {
	if annots == nil {
		annots = make(map[string]string)
	}
	// Update cache with latest template labels
	annots[cacheKey] = SerializeMap(template)
	return annots
}

// SerializeMap convert a map in a string of concatenated keys seprated by commas.
func SerializeMap(m map[string]string) string {
	serialized := ""
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		serialized += fmt.Sprintf("%s,", k)
	}
	return serialized
}

// DeSerializeCache splits a serialized map.
func DeSerializeCache(s string) []string {
	return strings.Split(s, ",")
}

// GetNestedField returns the nested field of a map.
// Example: GetNestedField(map[string]any{"a": map[string]any{"b": "c"}}, "a.b") returns "c".
func GetNestedField(m map[string]any, path string) (any, error) {
	fields := strings.Split(path, ".")
	current := m
	for i, field := range fields {
		next, ok := current[field]
		if !ok {
			return nil, fmt.Errorf("unable to get %s", strings.Join(fields[:i+1], "."))
		}
		if i == len(fields)-1 {
			return next, nil
		}
		current, ok = next.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unable to get %s", strings.Join(fields[:i+1], "."))
		}
	}
	return current, nil
}

// SliceToMap takes a slice of a generic type and returns a map where the keys are the array elements.
func SliceToMap[T comparable](slice []T) map[T]any {
	result := make(map[T]any)
	for _, elem := range slice {
		result[elem] = nil
	}
	return result
}
