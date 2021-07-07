// Package testutils encapsulates all methods and constants to perform E2E tests
package testutils

import "github.com/liqotech/liqo/pkg/consts"

const (
	liqoTestingLabelKey = "liqo.io/testing-namespace"
)

// LiqoTestNamespaceLabels is a set of labels that has to be attached to test namespaces to simplify garbage collection.
var LiqoTestNamespaceLabels = map[string]string{
	liqoTestingLabelKey:      "true",
	consts.EnablingLiqoLabel: consts.EnablingLiqoLabelValue,
}
