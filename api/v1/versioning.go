package v1

import (
	"strings"
)

const DEFAULT_PREFECT_VERSION = "3.9.1"
const DEFAULT_PREFECT_IMAGE = "prefecthq/prefect:" + DEFAULT_PREFECT_VERSION + "-python3.12"

func VersionFromImage(image string) string {
	parts := strings.Split(image, ":")
	if len(parts) < 2 {
		return image
	}
	if strings.ToLower(parts[0]) != "prefecthq/prefect" {
		return image
	}

	tag := parts[1]
	parts = strings.Split(tag, "-")

	return parts[0]
}
