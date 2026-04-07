package main

import "testing"

func TestNewerReleaseAvailable(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{name: "patch update", current: "v1.2.1-dev", latest: "v1.2.2", want: true},
		{name: "same version", current: "v1.2.1-dev", latest: "v1.2.1", want: false},
		{name: "older latest", current: "v1.2.1", latest: "v1.2.0", want: false},
		{name: "minor update", current: "v1.2.1", latest: "v1.3.0", want: true},
		{name: "major update", current: "v1.2.1", latest: "v2.0.0", want: true},
		{name: "unparseable current", current: "dev-build", latest: "v1.2.2", want: true},
		{name: "unparseable latest", current: "v1.2.1", latest: "latest", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newerReleaseAvailable(tt.current, tt.latest); got != tt.want {
				t.Fatalf("newerReleaseAvailable(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestNormalizeReleaseVersion(t *testing.T) {
	if got := normalizeReleaseVersion("v1.2.1-dev"); got != "v1.2.1" {
		t.Fatalf("expected normalized dev version, got %q", got)
	}
	if got := normalizeReleaseVersion("1.2.3"); got != "v1.2.3" {
		t.Fatalf("expected normalized plain version, got %q", got)
	}
	if got := normalizeReleaseVersion("not-a-version"); got != "" {
		t.Fatalf("expected empty normalized version, got %q", got)
	}
}
