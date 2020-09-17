package version

import (
	"fmt"
	"runtime"
)

var (
	// GitVersion returns the git version
	GitVersion = "UNKNOWN"
	// BuildDate returns the build date
	BuildDate = "UNKNOWN"
	// GitCommit returns the short sha from git
	GitCommit = "UNKNOWN"
)

// version returns information about the release.
func Version() string {
	return fmt.Sprintf(`-------------------------------------------------------------------------------
Meliodas cni plugin
  GitVersion: %v
  GitCommit:  %v
  BuildDate:  %v
  Go:         %v
-------------------------------------------------------------------------------
`, GitVersion, GitCommit, BuildDate, runtime.Version())
}