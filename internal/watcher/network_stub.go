//go:build !darwin

package watcher

type networkConnectionSample struct {
	RXBytes uint64
	TXBytes uint64
}

type networkObservation struct {
	PID      int32
	Domain   string
	Protocol string
	Sample   networkConnectionSample
}

func collectNetworkSamples() (map[string]networkObservation, error) {
	return map[string]networkObservation{}, nil
}
