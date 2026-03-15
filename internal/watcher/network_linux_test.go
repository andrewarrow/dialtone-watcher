//go:build linux

package watcher

import (
	"syscall"
	"testing"
)

func TestCollectNetworkSamplesLinuxDoesNotFail(t *testing.T) {
	observations, err := collectNetworkSamples()
	if err != nil {
		t.Fatalf("collectNetworkSamples() error = %v", err)
	}
	if observations == nil {
		t.Fatal("collectNetworkSamples() returned nil map")
	}
}

func TestParseLinuxSocketByteSamples(t *testing.T) {
	raw := `ESTAB 0 0 192.168.1.10:45218 104.16.132.229:443 users:(("curl",pid=1234,fd=7)) cubic bytes_received:2048 bytes_acked:5120`

	samples := parseLinuxSocketByteSamples(raw)
	key := "1234|7|1|192.168.1.10|45218|104.16.132.229|443"
	sample, ok := samples[key]
	if !ok {
		t.Fatalf("parseLinuxSocketByteSamples() missing key %q", key)
	}
	if sample.RXBytes != 2048 || sample.TXBytes != 5120 {
		t.Fatalf("sample = (%d, %d), want (2048, 5120)", sample.RXBytes, sample.TXBytes)
	}
}

func TestParseLinuxSocketUsersMultipleProcesses(t *testing.T) {
	line := `ESTAB 0 0 10.0.0.2:12345 1.1.1.1:443 users:(("proc-a",pid=10,fd=4),("proc-b",pid=11,fd=9)) cubic bytes_received:12 bytes_acked:34`

	users := parseLinuxSocketUsers(line)
	if len(users) != 2 {
		t.Fatalf("len(parseLinuxSocketUsers()) = %d, want 2", len(users))
	}
	if users[0].pid != 10 || users[0].fd != 4 || users[0].socketType != syscall.SOCK_STREAM {
		t.Fatalf("first user = %+v", users[0])
	}
	if users[1].pid != 11 || users[1].fd != 9 || users[1].socketType != syscall.SOCK_STREAM {
		t.Fatalf("second user = %+v", users[1])
	}
}

func TestParseLinuxSocketEndpointIPv6(t *testing.T) {
	host, port, ok := parseLinuxSocketEndpoint("[2606:4700:4700::1111]:853")
	if !ok {
		t.Fatal("parseLinuxSocketEndpoint() returned ok=false")
	}
	if host != "2606:4700:4700::1111" || port != 853 {
		t.Fatalf("parseLinuxSocketEndpoint() = (%q, %d), want (%q, %d)", host, port, "2606:4700:4700::1111", 853)
	}
}
