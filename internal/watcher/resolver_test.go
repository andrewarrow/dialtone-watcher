package watcher

import "testing"

func TestParseWhoisDomainPrefersRipeDomain(t *testing.T) {
	raw := `% IANA WHOIS server
refer:        whois.ripe.net

inetnum:      193.0.0.0 - 193.255.255.255
organisation: RIPE NCC

# whois.ripe.net
abuse-mailbox:  abuse@ripe.net
tech-c:         OPS4-RIPE
source:         RIPE # Filtered
`

	got := parseWhoisDomain(raw)
	if got != "ripe.net" {
		t.Fatalf("parseWhoisDomain() = %q, want %q", got, "ripe.net")
	}
}
