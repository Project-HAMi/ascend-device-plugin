// verify_chart_version verifies that a Helm Chart version is valid SemVer and
// strictly greater than its previous version.
package main

import (
	"fmt"
	"os"

	"golang.org/x/mod/semver"
)

func main() {
	if len(os.Args) != 3 {
		fail("usage: verify_chart_version.go <previous-version> <current-version>")
	}

	previous, current := os.Args[1], os.Args[2]
	if !semver.IsValid("v" + current) {
		fail("invalid current Chart version %q: must be SemVer", current)
	}

	if previous == "" {
		fmt.Printf("Initial Chart version: %s\n", current)
		return
	}
	if !semver.IsValid("v" + previous) {
		fail("invalid previous Chart version %q: must be SemVer", previous)
	}
	if semver.Compare("v"+previous, "v"+current) >= 0 {
		fail("Chart version must strictly increase: %s -> %s", previous, current)
	}

	fmt.Printf("Chart version: %s -> %s\n", previous, current)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
