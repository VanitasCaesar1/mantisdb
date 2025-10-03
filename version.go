package main

import (
	"fmt"
	"runtime"
)

var (
	// Version is set during build time
	Version = "dev"
	// BuildTime is set during build time
	BuildTime = "unknown"
	// GitCommit is set during build time
	GitCommit = "unknown"
)

// VersionInfo contains version information
type VersionInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// GetVersionInfo returns version information
func GetVersionInfo() VersionInfo {
	return VersionInfo{
		Version:   Version,
		BuildTime: BuildTime,
		GitCommit: GitCommit,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// PrintVersion prints version information
func PrintVersion() {
	info := GetVersionInfo()
	fmt.Printf("MantisDB %s\n", info.Version)
	fmt.Printf("Build Time: %s\n", info.BuildTime)
	fmt.Printf("Git Commit: %s\n", info.GitCommit)
	fmt.Printf("Go Version: %s\n", info.GoVersion)
	fmt.Printf("Platform: %s\n", info.Platform)
}
