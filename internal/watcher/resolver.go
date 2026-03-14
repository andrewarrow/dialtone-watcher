package watcher

import (
	"bytes"
	"context"
	"net"
	"net/netip"
	"net/url"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"
)

type reverseLookupResult struct {
	ready bool
	name  string
}

func startReverseLookupWorker(svc *service) chan<- string {
	queue := make(chan string, 256)

	go func() {
		for ip := range queue {
			name := lookupAddrName(ip)

			svc.mu.Lock()
			svc.lookupPending[ip] = false
			if name != "" {
				svc.lookupCache[ip] = reverseLookupResult{
					ready: true,
					name:  name,
				}
			} else {
				delete(svc.lookupCache, ip)
			}
			if record := svc.domains[ip]; record != nil {
				record.DisplayName = name
			}
			for _, record := range svc.connections {
				if record.Domain == ip {
					record.DisplayName = name
				}
			}
			_ = svc.persist(true)
			svc.mu.Unlock()
		}
	}()

	return queue
}

func lookupAddrName(ip string) string {
	if name := lookupPTRName(ip); name != "" {
		return name
	}

	if name := lookupHostCommandName(ip); name != "" {
		return name
	}

	if name := lookupDigCommandName(ip); name != "" {
		return name
	}

	if name := lookupWhoisName(ip); name != "" {
		return name
	}

	if name := lookupTLSCertName(ip); name != "" {
		return name
	}

	if name := lookupHTTPHeaderName(ip); name != "" {
		return name
	}

	return ""
}

func lookupHostCommandName(ip string) string {
	path, err := exec.LookPath("host")
	if err != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, path, ip).Output()
	if err != nil || ctx.Err() != nil {
		return ""
	}

	return parsePTROutput(string(output))
}

func lookupDigCommandName(ip string) string {
	path, err := exec.LookPath("dig")
	if err != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, path, "-x", ip, "+short").Output()
	if err != nil || ctx.Err() != nil {
		return ""
	}

	return parsePTROutput(string(output))
}

func parsePTROutput(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.LastIndex(line, "pointer"); idx != -1 {
			line = strings.TrimSpace(line[idx+len("pointer"):])
		}
		if host := normalizeResolvedHost(line); host != "" {
			return host
		}
	}

	return ""
}

func lookupPTRName(ip string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil {
		return ""
	}

	for _, name := range names {
		name = normalizeResolvedHost(name)
		if name != "" {
			return name
		}
	}

	return ""
}

var (
	whoisURLPattern      = regexp.MustCompile(`https?://[^\s)>"']+`)
	whoisHostnamePattern = regexp.MustCompile(`\b(?:[a-z0-9-]+\.)+[a-z]{2,}\b`)
	whoisEmailDomainPat  = regexp.MustCompile(`@[a-z0-9.-]+\.[a-z]{2,}\b`)
	whoisIgnoredDomains  = map[string]struct{}{
		"iana.org":    {},
		"arin.net":    {},
		"apnic.net":   {},
		"afrinic.net": {},
		"lacnic.net":  {},
	}
)

func lookupWhoisName(ip string) string {
	path, err := exec.LookPath("whois")
	if err != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, path, ip).Output()
	if err != nil || ctx.Err() != nil {
		return ""
	}

	return parseWhoisDomain(string(output))
}

func parseWhoisDomain(raw string) string {
	if domain := bestWhoisCandidate(whoisEmailDomainPat.FindAllString(raw, -1), func(match string) string {
		return strings.TrimPrefix(match, "@")
	}); domain != "" {
		return domain
	}

	for _, match := range whoisURLPattern.FindAllString(raw, -1) {
		parsed, err := url.Parse(match)
		if err != nil {
			continue
		}
		if domain := bestWhoisCandidate([]string{parsed.Hostname()}, func(match string) string {
			return match
		}); domain != "" {
			return domain
		}
	}

	return bestWhoisCandidate(whoisHostnamePattern.FindAllString(raw, -1), func(match string) string {
		return match
	})
}

func bestWhoisCandidate(matches []string, extractHost func(string) string) string {
	counts := make(map[string]int)

	for _, match := range matches {
		host := registrableDomain(extractHost(match))
		if host == "" {
			continue
		}
		if _, ignored := whoisIgnoredDomains[host]; ignored {
			continue
		}
		counts[host]++
	}

	bestHost := ""
	bestCount := 0
	for host, count := range counts {
		if count > bestCount || (count == bestCount && host < bestHost) {
			bestHost = host
			bestCount = count
		}
	}

	return bestHost
}

func lookupTLSCertName(ip string) string {
	opensslPath, err := exec.LookPath("openssl")
	if err != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sClient := exec.CommandContext(ctx, opensslPath, "s_client", "-connect", net.JoinHostPort(ip, "443"), "-servername", ip)
	sClient.Stdin = strings.NewReader("")
	sClient.Stderr = nil
	certOutput, err := sClient.Output()
	if err != nil || ctx.Err() != nil || len(certOutput) == 0 {
		return ""
	}

	x509 := exec.CommandContext(ctx, opensslPath, "x509", "-noout", "-ext", "subjectAltName")
	x509.Stdin = bytes.NewReader(certOutput)
	sanOutput, err := x509.Output()
	if err != nil || ctx.Err() != nil {
		return ""
	}

	return parseTLSSAN(string(sanOutput))
}

func parseTLSSAN(raw string) string {
	parts := strings.Split(raw, "DNS:")
	candidates := make([]string, 0, len(parts))

	for _, part := range parts[1:] {
		host := part
		if idx := strings.Index(host, ","); idx != -1 {
			host = host[:idx]
		}
		host = strings.TrimSpace(host)
		if strings.HasPrefix(host, "*.") {
			host = strings.TrimPrefix(host, "*.")
		}
		host = normalizeResolvedHost(host)
		if host == "" {
			continue
		}
		candidates = append(candidates, host)
	}

	return bestHostnameCandidate(candidates)
}

func lookupHTTPHeaderName(ip string) string {
	curlPath, err := exec.LookPath("curl")
	if err != nil {
		return ""
	}

	for _, target := range []string{"http://" + ip, "https://" + ip} {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		output, err := exec.CommandContext(
			ctx,
			curlPath,
			"-sS",
			"-k",
			"-I",
			"--max-time",
			"4",
			target,
		).Output()
		cancel()
		if err != nil || ctx.Err() != nil {
			continue
		}
		if host := parseHTTPHeadersForHost(string(output)); host != "" {
			return host
		}
	}

	return ""
}

func parseHTTPHeadersForHost(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(line), "location:") {
			continue
		}
		value := strings.TrimSpace(line[len("location:"):])
		parsed, err := url.Parse(value)
		if err != nil {
			continue
		}
		if host := normalizeResolvedHost(parsed.Hostname()); host != "" {
			return host
		}
	}

	return ""
}

func bestHostnameCandidate(hosts []string) string {
	if len(hosts) == 0 {
		return ""
	}

	unique := make(map[string]struct{}, len(hosts))
	filtered := make([]string, 0, len(hosts))
	for _, host := range hosts {
		if host == "" {
			continue
		}
		if _, seen := unique[host]; seen {
			continue
		}
		unique[host] = struct{}{}
		filtered = append(filtered, host)
	}

	slices.SortFunc(filtered, func(a, b string) int {
		aWildcardish := registrableDomain(a) == a
		bWildcardish := registrableDomain(b) == b
		if aWildcardish != bWildcardish {
			if !aWildcardish {
				return -1
			}
			return 1
		}
		if len(a) != len(b) {
			return len(a) - len(b)
		}
		return strings.Compare(a, b)
	})

	return filtered[0]
}

func normalizeResolvedHost(host string) string {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	host = strings.Trim(host, "[]")
	if host == "" {
		return ""
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if !isPublicIP(host) {
			return ""
		}
		return addr.String()
	}
	return host
}

func registrableDomain(host string) string {
	host = normalizeResolvedHost(host)
	if host == "" {
		return ""
	}

	labels := strings.Split(host, ".")
	if len(labels) <= 2 {
		return host
	}

	last := labels[len(labels)-1]
	secondLast := labels[len(labels)-2]
	if len(last) == 2 {
		switch secondLast {
		case "ac", "co", "com", "edu", "gov", "net", "org":
			return strings.Join(labels[len(labels)-3:], ".")
		}
	}

	return strings.Join(labels[len(labels)-2:], ".")
}

func isPublicIP(host string) bool {
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}

	return !(addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified())
}
