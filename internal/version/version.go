package version

import "runtime/debug"

// Version is set at build time via -ldflags.
// Default "dev" is used for go run and go install without flags.
var Version = "dev"

// Current returns the effective version string.
// If Version is non-empty and not "dev", it is returned as-is (ldflags override).
// Otherwise it falls back to the Go build info main module version,
// which is populated for tagged go installs (e.g. go install ...@v1.2.3).
func Current() string {
	if Version != "" && Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return Version
}
