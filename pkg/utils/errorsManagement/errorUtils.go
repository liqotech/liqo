package errorsmanagement

import "k8s.io/klog/v2"

var panicOnErrorMode = false

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
