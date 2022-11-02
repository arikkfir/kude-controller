package internal

import "github.com/blang/semver"

var versionString = "0.0.0-dev+unknown"
var version semver.Version

func init() {
	version = semver.MustParse(versionString)
}

// GetVersion returns the kude version currently running.
func GetVersion() semver.Version {
	return version
}
