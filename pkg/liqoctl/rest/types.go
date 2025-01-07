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

package rest

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
)

// APIOptions contains the options for the API.
type APIOptions struct {
	EnableCreate   bool
	EnableDelete   bool
	EnableGet      bool
	EnableUpdate   bool
	EnableGenerate bool
}

// CreateOptions contains the options for the create API.
type CreateOptions struct {
	*factory.Factory

	OutputFormat string
	Name         string
}

// DeleteOptions contains the options for the delete API.
type DeleteOptions struct {
	*factory.Factory

	Name string
}

// GetOptions contains the options for the get API.
type GetOptions struct {
	*factory.Factory

	OutputFormat string
	Name         string
}

// UpdateOptions contains the options for the update API.
type UpdateOptions struct {
	*factory.Factory
}

// GenerateOptions contains the options for the generate API.
type GenerateOptions struct {
	*factory.Factory

	OutputFormat string
}

// API is the interface that must be implemented by the API.
type API interface {
	APIOptions() *APIOptions
	Create(ctx context.Context, options *CreateOptions) *cobra.Command
	Delete(ctx context.Context, options *DeleteOptions) *cobra.Command
	Get(ctx context.Context, options *GetOptions) *cobra.Command
	Update(ctx context.Context, options *UpdateOptions) *cobra.Command
	Generate(ctx context.Context, options *GenerateOptions) *cobra.Command
}

// APIProvider is the function that returns the API.
type APIProvider func() API
