package main

import (
	"github.com/zdyxry/tokui/cmd"
)

// version is the tokui version string. It defaults to "dev" for local builds
// and is overridden at release time via -ldflags "-X main.version=<tag>"
// (goreleaser injects it into main.version by default).
var version = "dev"

func main() {
	cmd.Execute(version)
}
