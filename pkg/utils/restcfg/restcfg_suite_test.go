package restcfg_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRestcfg(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RestConfig Suite")
}
