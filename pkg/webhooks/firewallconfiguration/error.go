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

package firewallconfiguration

import (
	"fmt"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

// ChainError is an error that occurs during the validation of a chain.
type ChainError struct {
	chain *firewallapi.Chain
	err   error
}

func (ce *ChainError) Error() string {
	return fmt.Sprintf("chain %s: %s", *ce.chain.Name, ce.err.Error())
}

func forgeChainError(chain *firewallapi.Chain, err error) error {
	if chain.Name == nil {
		return fmt.Errorf("error forging ChainError: chain name is nil")
	}
	return &ChainError{chain: chain, err: err}
}
