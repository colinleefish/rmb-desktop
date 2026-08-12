package version

// Set at link time via -ldflags; defaults are for local dev builds.
var (
	Version = "0.1.13"
	Commit  = "dev"
)
