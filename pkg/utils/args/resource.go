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

package args

import "k8s.io/apimachinery/pkg/api/resource"

// Quantity implements the flag.Value interface and allows to parse strings expressing resource quantities.
type Quantity struct {
	Quantity resource.Quantity
}

// NewQuantity returns a new Quantity object initialized with the given resource quantity.
func NewQuantity(quantity string) Quantity {
	return Quantity{Quantity: resource.MustParse(quantity)}
}

// String returns the stringified quantity.
func (q *Quantity) String() string {
	return q.Quantity.String()
}

// Set parses the provided string as a resource quantity.
func (q *Quantity) Set(str string) error {
	quantity, err := resource.ParseQuantity(str)
	if err != nil {
		return err
	}
	q.Quantity = quantity
	return nil
}

// Type returns the quantity type.
func (q *Quantity) Type() string {
	return "quantity"
}
