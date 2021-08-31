package utils

import (
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// LogInfoLevel -> level associated with informational messages.
	LogInfoLevel = 0
	// LogDebugLevel -> level associated with debug messages.
	LogDebugLevel = 4
)

// FromResult returns a logger level, given the result of a CreateOrUpdate operation.
func FromResult(result controllerutil.OperationResult) klog.Level {
	if result == controllerutil.OperationResultNone {
		return LogDebugLevel
	}
	return LogInfoLevel
}
