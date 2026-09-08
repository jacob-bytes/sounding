// Package version 版本信息（构建时通过 ldflags 注入）。
package version

// Version 由 -ldflags "-X .../version.Version=vX.Y.Z" 注入。
var Version = "dev"

// Commit 构建提交（可选注入）。
var Commit = ""

// String 返回可读版本号。
func String() string {
	if Commit != "" {
		return Version + " (" + Commit + ")"
	}
	return Version
}
