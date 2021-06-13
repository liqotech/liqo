package iptables

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIptables(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Iptables Suite")
}
