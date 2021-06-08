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
	NotNil = "have to be not nil"
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
	Reason    string
	Parameter string
}

func (wp *WrongParameter) Error() string {
	return strings.Join([]string{wp.Parameter, " must be ", wp.Reason}, "")
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
