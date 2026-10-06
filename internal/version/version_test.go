package version

import "testing"

func TestTitleAppendsVersion(t *testing.T) {
	previous := Version
	defer func() { Version = previous }()
	Version = "v9.9.9"

	if Title() != "Cloud Command v9.9.9" {
		t.Fatalf("got %q", Title())
	}
}

func TestDefaultVersionIsDevelopment(t *testing.T) {
	if Version != DevelopmentVersion {
		t.Fatalf("got %q, want %q", Version, DevelopmentVersion)
	}
}
