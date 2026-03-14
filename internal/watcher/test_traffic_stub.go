//go:build !linux

package watcher

func startTestTraffic(enabled bool) func() {
	return func() {}
}
