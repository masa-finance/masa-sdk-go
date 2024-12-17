package x_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestX(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "X Integration Suite")
}
