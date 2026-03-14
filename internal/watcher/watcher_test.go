package watcher

import "testing"

func TestTopDomainsSkipsUnresolvedPublicIP(t *testing.T) {
	svc := &service{
		domains: map[string]*domainRecord{
			"193.0.19.229": {
				DomainSnapshot: DomainSnapshot{
					Domain:    "193.0.19.229",
					RXBytes:   1024,
					TXBytes:   2048,
					PollsSeen: 3,
				},
			},
		},
	}

	if got := svc.topDomains(6); len(got) != 0 {
		t.Fatalf("topDomains() returned %d records, want 0", len(got))
	}
}

func TestTopDomainsIncludesResolvedPublicIP(t *testing.T) {
	svc := &service{
		domains: map[string]*domainRecord{
			"193.0.19.229": {
				DomainSnapshot: DomainSnapshot{
					Domain:      "193.0.19.229",
					DisplayName: "ripe.net",
					RXBytes:     1024,
					TXBytes:     2048,
					PollsSeen:   3,
				},
			},
		},
	}

	got := svc.topDomains(6)
	if len(got) != 1 {
		t.Fatalf("topDomains() returned %d records, want 1", len(got))
	}
	if got[0].DisplayName != "ripe.net" {
		t.Fatalf("topDomains()[0].DisplayName = %q, want %q", got[0].DisplayName, "ripe.net")
	}
}

func TestTopConnectionsSortsByTraffic(t *testing.T) {
	svc := &service{
		connections: map[string]*connectionRecord{
			"1": {
				ConnectionSnapshot: ConnectionSnapshot{
					PID:         101,
					ProcessName: "Chrome",
					Domain:      "github.com",
					Protocol:    "HTTPS",
					RXBytes:     1024,
					TXBytes:     512,
					PollsSeen:   2,
				},
			},
			"2": {
				ConnectionSnapshot: ConnectionSnapshot{
					PID:         202,
					ProcessName: "Slack",
					Domain:      "slack.com",
					Protocol:    "HTTPS",
					RXBytes:     4096,
					TXBytes:     2048,
					PollsSeen:   1,
				},
			},
		},
	}

	got := svc.topConnections(2)
	if len(got) != 2 {
		t.Fatalf("topConnections() returned %d records, want 2", len(got))
	}
	if got[0].ProcessName != "Slack" {
		t.Fatalf("topConnections()[0].ProcessName = %q, want %q", got[0].ProcessName, "Slack")
	}
	if got[1].ProcessName != "Chrome" {
		t.Fatalf("topConnections()[1].ProcessName = %q, want %q", got[1].ProcessName, "Chrome")
	}
}
