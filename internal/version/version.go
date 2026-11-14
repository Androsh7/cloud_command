// Package version exposes the build version stamped in by the Makefile.
package version

// DevelopmentVersion is shown by builds that were not stamped with a release version.
const DevelopmentVersion = "DEVELOPMENT"

// Version is set at build time with -ldflags "-X .../internal/version.Version=v1.2.3".
var Version = DevelopmentVersion

// Title returns the application name with the version appended.
func Title() string {
	return "Cloud Command " + Version
}
