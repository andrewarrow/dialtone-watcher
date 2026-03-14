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

func TestInferProtocol(t *testing.T) {
	tests := []struct {
		port uint16
		want string
	}{
		{port: 80, want: "HTTP"},
		{port: 443, want: "HTTPS"},
		{port: 53, want: "DNS"},
		{port: 5432, want: "Postgres"},
		{port: 9999, want: "TCP"},
	}

	for _, tt := range tests {
		if got := inferProtocol(tt.port); got != tt.want {
			t.Fatalf("inferProtocol(%d) = %q, want %q", tt.port, got, tt.want)
		}
	}
}
