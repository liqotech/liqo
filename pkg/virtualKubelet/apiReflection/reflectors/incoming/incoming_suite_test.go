package incoming_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIncoming(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Incoming Suite")
}
