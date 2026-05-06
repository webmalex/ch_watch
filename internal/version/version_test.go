package version

import "testing"

func TestCurrentReturnsVersionWhenSet(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "1.2.3"
	if got := Current(); got != "1.2.3" {
		t.Errorf("Current() = %q, want %q", got, "1.2.3")
	}
}

func TestCurrentFallsBackWhenDev(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "dev"
	got := Current()
	if got != "dev" {
		t.Errorf("Current() = %q, want %q (fallback to dev when no build info)", got, "dev")
	}
}

func TestCurrentFallsBackWhenEmpty(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = ""
	got := Current()
	if got != "" {
		t.Errorf("Current() = %q, want %q (fallback to empty when no build info)", got, "")
	}
}
