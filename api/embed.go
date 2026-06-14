// Package api exposes the embedded OpenAPI specification.
package api

import _ "embed"

//go:embed openapi.yaml
var SpecYAML []byte
