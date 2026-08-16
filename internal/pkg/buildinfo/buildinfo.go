// Package buildinfo exposes immutable identity compiled into a Chatz binary.
package buildinfo

const DevelopmentVersion = "dev"

//nolint:gochecknoglobals // build flags replace process facts.
var (
	// Version is replaced by the production image build for tagged releases.
	Version = DevelopmentVersion

	// Commit is replaced by the production image build when its source revision
	// is available. Local runs retain the empty value.
	Commit = ""
)
