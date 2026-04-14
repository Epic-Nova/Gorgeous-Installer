package buildinfo

import "fmt"

// These values are injected at build time via -ldflags.
var (
	CommitSHA = "dev"
	BuildTime = "unknown"
)

func Summary() string {
	return fmt.Sprintf("commit=%s build_time=%s", CommitSHA, BuildTime)
}
