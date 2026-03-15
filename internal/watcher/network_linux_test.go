//go:build linux

package watcher

import (
	"strings"
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

func TestParseLinuxConntrackLineUDP(t *testing.T) {
	line := "ipv4 2 udp 17 29 src=192.168.1.10 dst=1.1.1.1 sport=53000 dport=53 packets=2 bytes=140 src=1.1.1.1 dst=192.168.1.10 sport=53 dport=53000 packets=2 bytes=220 mark=0 use=1"

	samples := parseLinuxConntrackLine(line)
	key := linuxFlowTupleKey(syscall.SOCK_DGRAM, "192.168.1.10", 53000, "1.1.1.1", 53)
	sample, ok := samples[key]
	if !ok {
		t.Fatalf("parseLinuxConntrackLine() missing key %q", key)
	}
	if sample.RXBytes != 220 || sample.TXBytes != 140 {
		t.Fatalf("sample = (%d, %d), want (220, 140)", sample.RXBytes, sample.TXBytes)
	}
}

func TestParseLinuxConntrackSamplesSkipsNonUDP(t *testing.T) {
	raw := strings.NewReader(strings.Join([]string{
		"ipv4 2 tcp 6 431999 ESTABLISHED src=192.168.1.10 dst=104.16.132.229 sport=45218 dport=443 packets=3 bytes=280 src=104.16.132.229 dst=192.168.1.10 sport=443 dport=45218 packets=2 bytes=2048 mark=0 use=1",
		"ipv4 2 udp 17 29 src=192.168.1.10 dst=8.8.8.8 sport=54000 dport=53 packets=1 bytes=52 src=8.8.8.8 dst=192.168.1.10 sport=53 dport=54000 packets=1 bytes=168 mark=0 use=1",
	}, "\n"))

	samples := parseLinuxConntrackSamples(raw)
	if len(samples) != 2 {
		t.Fatalf("len(parseLinuxConntrackSamples()) = %d, want 2", len(samples))
	}
	key := linuxFlowTupleKey(syscall.SOCK_DGRAM, "192.168.1.10", 54000, "8.8.8.8", 53)
	if _, ok := samples[key]; !ok {
		t.Fatalf("parseLinuxConntrackSamples() missing UDP key %q", key)
	}
}
