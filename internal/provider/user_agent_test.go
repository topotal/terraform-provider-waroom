package provider

import (
	"runtime/debug"
	"testing"
)

func TestUserAgent(t *testing.T) {
	tests := []struct {
		name string
		bi   *debug.BuildInfo
		ok   bool
		want string
	}{
		{
			name: "module version",
			bi:   &debug.BuildInfo{Deps: []*debug.Module{{Path: clientGoModule, Version: "v0.0.7"}}},
			ok:   true,
			want: "terraform-provider-waroom/1.2.3 waroom-client-go/0.0.7",
		},
		{
			name: "replaced by another version",
			bi: &debug.BuildInfo{Deps: []*debug.Module{{
				Path: clientGoModule, Version: "v0.0.7",
				Replace: &debug.Module{Path: "example.com/fork", Version: "v0.0.8-fork"},
			}}},
			ok:   true,
			want: "terraform-provider-waroom/1.2.3 waroom-client-go/0.0.8-fork",
		},
		{
			name: "local replacement",
			bi: &debug.BuildInfo{Deps: []*debug.Module{{
				Path: clientGoModule, Version: "v0.0.7",
				Replace: &debug.Module{Path: "../waroom-client-go"},
			}}},
			ok:   true,
			want: "terraform-provider-waroom/1.2.3 waroom-client-go/devel",
		},
		{
			name: "no build info",
			ok:   false,
			want: "terraform-provider-waroom/1.2.3 waroom-client-go/unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := readBuildInfo
			t.Cleanup(func() { readBuildInfo = orig })
			readBuildInfo = func() (*debug.BuildInfo, bool) { return tt.bi, tt.ok }

			if got := userAgent("1.2.3"); got != tt.want {
				t.Errorf("userAgent() = %q, want %q", got, tt.want)
			}
		})
	}
}
