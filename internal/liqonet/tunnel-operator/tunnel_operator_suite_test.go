package tunneloperator

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	tc = &TunnelController{}
)

func TestTunnelOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TunnelOperator Suite")
}
