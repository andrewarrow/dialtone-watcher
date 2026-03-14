//go:build linux

package watcher

import "testing"

func TestCollectNetworkSamplesLinuxDoesNotFail(t *testing.T) {
	observations, err := collectNetworkSamples()
	if err != nil {
		t.Fatalf("collectNetworkSamples() error = %v", err)
	}
	if observations == nil {
		t.Fatal("collectNetworkSamples() returned nil map")
	}
}
