// Package docker provides embedded Docker assets for the autopus-bridge.
package docker

import _ "embed"

// ChromiumSandboxDockerfile contains the embedded Dockerfile for building
// the chromium-sandbox image locally when a pull from the registry fails.
//
//go:embed chromium-sandbox/Dockerfile
var ChromiumSandboxDockerfile []byte
