// Package openapi embeds the API contract into the core binary.
//
// openapi.yml here is a copy of the repository root openapi.yml: the Docker build
// context is backend-go/, so the root file is not visible to go:embed.
// Update the copy with `make openapi`; TestCopyMatchesRoot fails if they differ.
package openapi

import _ "embed"

// Spec is the OpenAPI 3.1 document served at /api/v1/openapi.yml.
//
//go:embed openapi.yml
var Spec []byte
