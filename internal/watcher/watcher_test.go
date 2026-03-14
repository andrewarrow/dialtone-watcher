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
