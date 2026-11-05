package v1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Version parsing", func() {
	It("parses the version from an image", func() {
		Expect(VersionFromImage("prefecthq/prefect:latest")).To(Equal("latest"))
		Expect(VersionFromImage("prefecthq/prefect:0.0.1")).To(Equal("0.0.1"))
		Expect(VersionFromImage("prefecthq/prefect:0.0.1rc13")).To(Equal("0.0.1rc13"))
		Expect(VersionFromImage("prefecthq/prefect:0.0.1-foo-bar")).To(Equal("0.0.1"))
		Expect(VersionFromImage("prefecthq/prefect:0.0.1rc13-foo-bar")).To(Equal("0.0.1rc13"))
		Expect(VersionFromImage("whatdis")).To(Equal("whatdis"))
		Expect(VersionFromImage("hmmm:0.0.1")).To(Equal("hmmm:0.0.1"))
	})
})
