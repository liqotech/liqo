package apiserver_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestApiserver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Apiserver Suite")
}
