package errorsmanagement

import "k8s.io/klog/v2"

var panicMode = false

// SetPanicMode can be used to set or unset the panic mode.
func SetPanicMode(status bool) {
	panicMode = status
}

// Must wraps a function call that can return an error. If some error occurred Must has two possible behaviors:
// panic if debug = true or log the error and return false in order to recover the error.
// Returns true if no error occurred.
func Must(err error) bool {
	if err != nil {
		if panicMode {
			panic(err)
		} else {
			klog.Errorf("%s", err)
			return false
		}
	}
	return true
}
