package wireguard

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestWireguard(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wireguard Suite")
}
