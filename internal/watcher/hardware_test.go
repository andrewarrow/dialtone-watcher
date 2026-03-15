package watcher

import "testing"

func TestNormalizePlatformDarwinUsesMacOSLabel(t *testing.T) {
	if got := normalizePlatform("darwin", "darwin"); got != "macOS" {
		t.Fatalf("normalizePlatform() = %q, want %q", got, "macOS")
	}
}

func TestNormalizePlatformLeavesLinuxDistributionLabel(t *testing.T) {
	if got := normalizePlatform("linux", "ubuntu"); got != "ubuntu" {
		t.Fatalf("normalizePlatform() = %q, want %q", got, "ubuntu")
	}
}
