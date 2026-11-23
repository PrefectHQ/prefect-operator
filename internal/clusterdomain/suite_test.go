package clusterdomain

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestClusterDomain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cluster Domain Suite")
}
