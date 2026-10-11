package buildinfo

import "fmt"

// These values are injected by the release build; plain go build stays identifiable.
var Version = "dev"
var Commit = "unknown"
var Date = "unknown"

func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date)
}
