package iptables

import (
	"testing"

	. "github.com/coreos/go-iptables/iptables"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIptables(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Iptables Suite")
}

var _ = BeforeSuite(func() {
	var err error
	h, err = NewIPTHandler()
	Expect(err).To(BeNil())
	ipt, err = New()
	Expect(err).To(BeNil())
	err = h.Init()
	Expect(err).To(BeNil())
})

var _ = AfterSuite(func() {
	err := h.Terminate()
	Expect(err).To(BeNil())
})
