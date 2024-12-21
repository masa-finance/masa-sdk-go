package nineteen_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNineteen(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Nineteen Integration Suite")
}
