package errorsmanagement

import "k8s.io/klog/v2"

var debug = false

// SetDebug can be used to set or unset the debug mode.
func SetDebug(status bool) {
	debug = status
}

// Must wraps a function call that can return an error. If some error occurred Must has two possible behaviors:
// panic if debug = true or log the error and return false in order to recover the error.
// Returns true if no error occurred.
func Must(err error) bool {
	if err != nil {
		if debug {
			panic(err)
		} else {
			klog.Errorf("%s", err)
			return false
		}
	}
	return true
}
