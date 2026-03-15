package version

// Repo is the GitHub repository used for release downloads and upgrade checks.
const Repo = "roelentless/tko"

// Version is overridden at build time via:
//
//	go build -ldflags "-X tko/internal/version.Version=1.2.3"
var Version = "dev"
