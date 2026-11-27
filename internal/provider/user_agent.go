package provider

import (
	"runtime/debug"
	"strings"
)

const clientGoModule = "github.com/topotal/waroom-client-go"

// readBuildInfo is replaced in tests.
var readBuildInfo = debug.ReadBuildInfo

// userAgent returns the User-Agent sent to the Waroom API, naming the provider
// version and the waroom-client-go version it was built with.
func userAgent(providerVersion string) string {
	return "terraform-provider-waroom/" + providerVersion + " waroom-client-go/" + clientGoVersion()
}

// clientGoVersion returns the waroom-client-go version from the build info
// (the replacement's version when go.mod replaces it), "devel" for a local
// replacement or go.work checkout, and "unknown" without build info.
func clientGoVersion() string {
	bi, ok := readBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, m := range bi.Deps {
		if m.Path != clientGoModule {
			continue
		}
		if m.Replace != nil {
			m = m.Replace
		}
		if m.Version == "" || m.Version == "(devel)" {
			return "devel"
		}
		return strings.TrimPrefix(m.Version, "v")
	}
	return "unknown"
}
