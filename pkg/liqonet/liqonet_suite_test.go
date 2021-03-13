package liqonet_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLiqonet(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqonet Suite")
}
