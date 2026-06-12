// Command sjctl is the deterministic engine behind solid-jobs-skills: it
// fetches offers from the SOLID.Jobs public API, caches them in SQLite, and
// manages the application pipeline. Claude skills drive it; humans can too.
package main

import (
	"fmt"
	"os"
)

// version is the build version, injected at release time via
// -ldflags "-X main.version=v1.2.3". It stays "dev" for local builds.
var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
