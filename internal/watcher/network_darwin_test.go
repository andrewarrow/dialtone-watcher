//go:build darwin

package watcher

import "testing"

func TestParseEndpointPort(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     uint16
	}{
		{name: "ipv4", endpoint: "github.com:443", want: 443},
		{name: "ipv6", endpoint: "[2606:50c0:8000::153]:443", want: 443},
		{name: "nettop ipv6", endpoint: "2606:50c0:8000::153.443", want: 443},
		{name: "missing", endpoint: "github.com", want: 0},
	}

	for _, tt := range tests {
		if got := parseEndpointPort(tt.endpoint); got != tt.want {
			t.Fatalf("%s: parseEndpointPort(%q) = %d, want %d", tt.name, tt.endpoint, got, tt.want)
		}
	}
}
