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

package errors

import (
	"reflect"
	"strings"
)

const (
	// GreaterOrEqual used as reason of failure in WrongParameter error.
	GreaterOrEqual = ">="
	// MinorOrEqual used as reason of failure in WrongParameter error.
	MinorOrEqual = "<="
	// AtLeastOneValid used as reason of failure in WrongParameter error.
	AtLeastOneValid = "at least one of the arguments has to be valid"
	// ValidIP used as reason of failure in WrongParameter error.
	ValidIP = "a valid IP address"
	// NotNil used as reason of failure in WrongParameter error.
	NotNil = "not nil"
	// ValidCIDR used as reason of failure in WrongParameter error.
	ValidCIDR = "a valid network CIDR"
	// StringNotEmpty used as reason of failure in WrongParameter error.
	StringNotEmpty = "not empty"
	// Initialization used as reason of failure in WrongParameter error.
	Initialization = "initialized first"
)

// ParseIPError it is returned when net.ParseIP() fails to parse and ip address.
type ParseIPError struct {
	IPToBeParsed string
}

func (pie *ParseIPError) Error() string {
	return "please check that the IP address is in che correct format: " + pie.IPToBeParsed
}

// WrongParameter it is returned when parameters passed to a function are not correct.
type WrongParameter struct {
	Argument  string
	Reason    string
	Parameter string
}

func (wp *WrongParameter) Error() string {
	return strings.Join([]string{wp.Parameter, " must be ", wp.Reason, wp.Argument}, "")
}

// NoRouteFound it is returned when no route is found for a given destination network.
type NoRouteFound struct {
	IPAddress string
}

func (nrf *NoRouteFound) Error() string {
	return strings.Join([]string{"no route found for IP address: ", nrf.IPAddress}, "")
}

// MissingInit is returned when a data structure is tried to be used before correct
// initialization.
type MissingInit struct {
	StructureName string
}

func (sni *MissingInit) Error() string {
	return strings.Join([]string{sni.StructureName, "must be", Initialization}, " ")
}

// Is function is used for assert that a generic error is a MissingInit error.
func (sni *MissingInit) Is(target error) bool {
	return reflect.TypeOf(sni) == reflect.TypeOf(target)
}
