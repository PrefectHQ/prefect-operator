package v1

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/version"
)

// minServerAuthVersion is the first Prefect release whose server requires the
// auth string on every API request except its health checks. Servers before
// 3.1.8 ignore PREFECT_SERVER_API_AUTH_STRING.
var minServerAuthVersion = version.MustParseGeneric("3.6.22")

// ValidateServerAuthImage returns an error if image is a Prefect release that
// does not support PrefectServer.spec.auth. Otherwise, known reports whether
// image is a release that does.
//
// The release is the one in the tag of a prefecthq/prefect image, in any
// registry. It is not known for other images, for tags such as "3-latest" and
// "3.6-python3.12" that can be a release on either side of the first one that
// supports auth, or for an image with a digest, which may be a release other
// than the tag's.
func ValidateServerAuthImage(image string) (known bool, err error) {
	tag, ok := prefectImageTag(image)
	if !ok {
		return false, nil
	}
	release, _, _ := strings.Cut(tag, "-")
	if major, err := strconv.ParseUint(release, 10, 16); err == nil {
		if uint(major) < minServerAuthVersion.Major() {
			return false, unsupportedServerAuthImage(image)
		}
		return false, nil
	}
	v, err := version.ParseGeneric(release)
	if err != nil {
		return false, nil
	}
	if release == fmt.Sprintf("%d.%d", minServerAuthVersion.Major(), minServerAuthVersion.Minor()) {
		return false, nil
	}
	// ParseGeneric ignores a suffix such as "rc1" or ".dev8", which makes the
	// tag a release before the one without it.
	prerelease := v.EqualTo(minServerAuthVersion) && release != minServerAuthVersion.String()
	if v.LessThan(minServerAuthVersion) || prerelease {
		return false, unsupportedServerAuthImage(image)
	}
	return !strings.Contains(image, "@"), nil
}

func unsupportedServerAuthImage(image string) error {
	return fmt.Errorf("spec.auth requires Prefect %s or later, and the server's image is %q", minServerAuthVersion, image)
}

// UnknownServerAuthImage returns a message for a server with spec.auth whose
// image has no known release: see ValidateServerAuthImage.
func UnknownServerAuthImage(image string) string {
	return fmt.Sprintf("the operator can't tell the Prefect version of the image %q: a server before Prefect %s may accept requests without the auth string",
		image, minServerAuthVersion)
}

// prefectImageTag returns the tag of a prefecthq/prefect image. It returns
// false for other images and for an image without a tag.
func prefectImageTag(image string) (string, bool) {
	image, _, _ = strings.Cut(image, "@")
	dir, base := path.Split(strings.ToLower(image))
	name, tag, ok := strings.Cut(base, ":")
	if !ok || name != "prefect" || path.Base(dir) != "prefecthq" {
		return "", false
	}
	return tag, true
}
