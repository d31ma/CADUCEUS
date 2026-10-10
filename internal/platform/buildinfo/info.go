// Package buildinfo exposes immutable metadata about the running executable.
package buildinfo

import "runtime"

const (
	defaultVersion = "dev"
	defaultValue   = "unknown"
)

var (
	version = defaultVersion
	commit  = defaultValue
	builtAt = defaultValue
)

// Info identifies one CADUCEUS build.
type Info struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuiltAt   string `json:"built_at"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// Current returns metadata injected with linker flags at build time.
func Current() Info {
	return New(version, commit, builtAt)
}

// New constructs build metadata with explicit development defaults.
func New(releaseVersion, sourceCommit, buildTime string) Info {
	if releaseVersion == "" {
		releaseVersion = defaultVersion
	}
	if sourceCommit == "" {
		sourceCommit = defaultValue
	}
	if buildTime == "" {
		buildTime = defaultValue
	}
	return Info{
		Name:      "caduceus",
		Version:   releaseVersion,
		Commit:    sourceCommit,
		BuiltAt:   buildTime,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}
