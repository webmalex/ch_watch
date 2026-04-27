package version

// Version is set at build time via -ldflags.
// Default "dev" is used for go run and go install without flags.
var Version = "dev"
