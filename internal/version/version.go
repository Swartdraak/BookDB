// Package version carries the BookDB build version and related metadata.
//
// The values are set at build time via -ldflags (see the Makefile, .github
// workflows, and the Dockerfile). When built without ldflags, the defaults
// below are used. The version package is imported so the binary exposes a
// stable, deterministic identity for `bookdb version`.
package version

import "runtime"

// Build-time injectable fields (defaulted for local development).
var (
	// Version is the semantic version, e.g. "0.1.0".
	Version = "0.1.0"
	// Commit is the git commit SHA (full, 40 chars) or "HEAD".
	Commit = "HEAD"
	// Date is the UTC build timestamp (RFC 3339).
	Date = "unknown"
	// Channel is the release channel: "stable", "beta", or "dev".
	Channel = "dev"
)

// Info is the structured identity of a BookDB build.
type Info struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Commit   string `json:"commit"`
	Date     string `json:"date"`
	Channel  string `json:"channel"`
	Go       string `json:"go"`
	Platform string `json:"platform"`
}

// Get returns the current build identity.
func Get() Info {
	return Info{
		Name:     "bookdb",
		Version:  Version,
		Commit:   Commit,
		Date:     Date,
		Channel:  Channel,
		Go:       runtime.Version(),
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
	}
}
