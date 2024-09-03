/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"strings"
)

const DEFAULT_PREFECT_VERSION = "3.0.0"
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
