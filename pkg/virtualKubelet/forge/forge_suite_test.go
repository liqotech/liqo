package forge_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestForge(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Forge Suite")
}
