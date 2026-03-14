package watcher

import (
	"context"
	"net"
	"net/netip"
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
			svc.lookupCache[ip] = reverseLookupResult{
				ready: true,
				name:  name,
			}
			if record := svc.domains[ip]; record != nil {
				record.DisplayName = name
			}
			_ = svc.persist(true)
			svc.mu.Unlock()
		}
	}()

	return queue
}

func lookupAddrName(ip string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil {
		return ""
	}

	for _, name := range names {
		name = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
		if name != "" {
			return name
		}
	}

	return ""
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
