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

package utils

import (
	"fmt"
	"net"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

func ConvertSetData(data *string, dataType *firewallapi.SetDataType) ([]byte, error) {
	if dataType == nil {
		if data != nil {
			return nil, fmt.Errorf("set element has data but no data type is specified")
		}
		return []byte{}, nil
	}

	if data == nil {
		return nil, fmt.Errorf("set element has nil data for data type %s", *dataType)
	}

	switch *dataType {
	case firewallapi.SetDataTypeIPAddr:
		ip := net.ParseIP(*data)
		if ip == nil {
			return nil, fmt.Errorf("set element has invalid IP value %s", *data)
		}
		return ip.To4(), nil

	default:
		return nil, fmt.Errorf("invalid set value type %s", *dataType)
	}
}
