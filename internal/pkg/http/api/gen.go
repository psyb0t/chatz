// Package api holds the oapi-codegen output for the chatz HTTP API: the server
// interfaces, the request/response types, and the strict-handler wiring. This
// file is hand-maintained (it only declares the generator); api.gen.go next to
// it is the generated artifact.
//
//nolint:revive // Package name is fixed by the codegen config + import path.
package api

//go:generate go tool oapi-codegen --config=config.yaml ../../../../api/api.yml
