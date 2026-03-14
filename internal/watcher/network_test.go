package watcher

import (
	"syscall"
	"testing"
)

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

func TestInferSocketProtocol(t *testing.T) {
	tests := []struct {
		name       string
		socketType uint32
		port       uint32
		want       string
	}{
		{name: "tcp https", socketType: syscall.SOCK_STREAM, port: 443, want: "HTTPS"},
		{name: "udp dns", socketType: syscall.SOCK_DGRAM, port: 53, want: "DNS"},
		{name: "udp quic", socketType: syscall.SOCK_DGRAM, port: 443, want: "QUIC"},
		{name: "udp other", socketType: syscall.SOCK_DGRAM, port: 9999, want: "UDP"},
	}

	for _, tt := range tests {
		if got := inferSocketProtocol(tt.socketType, tt.port); got != tt.want {
			t.Fatalf("%s: inferSocketProtocol(%d, %d) = %q, want %q", tt.name, tt.socketType, tt.port, got, tt.want)
		}
	}
}
