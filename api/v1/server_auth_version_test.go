package v1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ValidateServerAuthImage", func() {
	DescribeTable("accepts",
		func(image string) {
			Expect(ValidateServerAuthImage(image)).To(BeTrue())
		},
		Entry("the default image", DEFAULT_PREFECT_IMAGE),
		Entry("the first release that supports auth", "prefecthq/prefect:3.6.22-python3.12"),
		Entry("a later minor version without a patch", "prefecthq/prefect:3.7-python3.12"),
		Entry("a prerelease of a later version", "prefecthq/prefect:3.6.23.dev1-python3.12"),
		Entry("another registry", "registry.example.com:5000/mirror/prefecthq/prefect:3.9.4"),
	)

	DescribeTable("doesn't know the release of",
		func(image string) {
			Expect(ValidateServerAuthImage(image)).To(BeFalse())
		},
		Entry("a major version", "prefecthq/prefect:3-latest"),
		Entry("the minor version of the first release that supports auth", "prefecthq/prefect:3.6-python3.12"),
		Entry("a tag that isn't a version", "prefecthq/prefect:latest"),
		Entry("a digest", "prefecthq/prefect@sha256:0123456789abcdef"),
		Entry("a tag with a digest", "prefecthq/prefect:3.9.4@sha256:0123456789abcdef"),
		Entry("an image without a tag", "prefecthq/prefect"),
		Entry("a development image", "prefecthq/prefect-dev:3.1.0"),
		Entry("a mirror without the organization", "registry.example.com/prefect:3.1.0"),
		Entry("an image with its own version numbers", "registry.example.com:5000/acme/prefect:1.0.0"),
	)

	DescribeTable("refuses",
		func(image string) {
			_, err := ValidateServerAuthImage(image)
			Expect(err).To(MatchError(
				`spec.auth requires Prefect 3.6.22 or later, and the server's image is "` + image + `"`))
		},
		Entry("the release before", "prefecthq/prefect:3.6.21-python3.12"),
		Entry("a release whose patch has fewer digits", "prefecthq/prefect:3.6.9"),
		Entry("an earlier minor version without a patch", "prefecthq/prefect:3.5-python3.12"),
		Entry("Prefect 2", "prefecthq/prefect:2.20.1-python3.12"),
		Entry("a major version", "prefecthq/prefect:2-latest"),
		Entry("a release candidate", "prefecthq/prefect:3.6.21rc1"),
		Entry("a release candidate of the first release that supports auth", "prefecthq/prefect:3.6.22rc1"),
		Entry("a development build of the first release that supports auth", "prefecthq/prefect:3.6.22.dev8-python3.12"),
		Entry("a tag with a digest", "prefecthq/prefect:3.1.0@sha256:0123456789abcdef"),
		Entry("another registry", "registry.example.com:5000/mirror/prefecthq/prefect:3.1.0"),
	)
})
