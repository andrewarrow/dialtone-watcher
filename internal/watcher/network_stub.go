//go:build !darwin && !linux

package watcher

func collectNetworkSamples() (map[string]networkObservation, error) {
	return map[string]networkObservation{}, nil
}
