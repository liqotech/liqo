package errorsmanagement

import (
	"flag"

	"k8s.io/klog/v2"
)

var panicOnErrorMode = false

// InitFlags initializes the flags to configure the errormanagement parameter.
func InitFlags(flagset *flag.FlagSet) {
	if flagset == nil {
		flagset = flag.CommandLine
	}

	flagset.BoolVar(&panicOnErrorMode, "panic-on-unexpected-errors", panicOnErrorMode,
		"Enable a pedantic mode which causes a panic if an unexpected error occurs")
}

// SetPanicOnErrorMode can be used to set or unset the panic mode.
func SetPanicOnErrorMode(status bool) {
	panicOnErrorMode = status
}

// Must wraps a function call that can return an error. If some error occurred Must has two possible behaviors:
// panic if debug = true or log the error and return false in order to recover the error.
// Returns true if no error occurred.
func Must(err error) bool {
	if err != nil {
		if panicOnErrorMode {
			panic(err)
		} else {
			klog.Errorf("%s", err)
			return false
		}
	}
	return true
}
