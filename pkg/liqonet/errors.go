package liqonet

import "strings"

const (
	// GreaterOrEqual used as reason of failure in WrongParameter error.
	GreaterOrEqual = ">="
	// MinorOrEqual used as reason of failure in WrongParameter error.
	MinorOrEqual = "<="
	// AtLeastOneValid used as reason of failure in WrongParameter error.
	AtLeastOneValid = "at least one of the arguments has to be valid"
	// StringNotEmpty used as reason of failure in WrongParameter error.
	StringNotEmpty = "not empty"
	// ValidIP used as reason of failure in WrongParameter error.
	ValidIP = "a valid IP address"
	// NotNil used as reason of failure in WrongParameter error.
	NotNil = "have to be not nil"
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
	if wp.Reason == StringNotEmpty {
		return strings.Join([]string{"Parameter must be ", wp.Reason}, "")
	}
	return strings.Join([]string{wp.Parameter, " must be ", wp.Reason}, "")
}

// NoRouteFound it is returned when no route is found for a given destination network.
type NoRouteFound struct {
	IPAddress string
}

func (nrf *NoRouteFound) Error() string {
	return strings.Join([]string{"no route found for IP address: ", nrf.IPAddress}, "")
}
