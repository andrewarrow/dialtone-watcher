//go:build linux

package watcher

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

var linuxTestTargets = []string{
	"google.com",
	"apple.com",
	"microsoft.com",
}

func startTestTraffic(enabled bool) func() {
	if !enabled {
		return func() {}
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	for _, host := range linuxTestTargets {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			runTestTrafficLoop(ctx, host)
		}(host)
	}

	return func() {
		cancel()
		wg.Wait()
	}
}

func runTestTrafficLoop(ctx context.Context, host string) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		driveTestConnection(ctx, host)

		timer.Reset(500 * time.Millisecond)
	}
}

func driveTestConnection(ctx context.Context, host string) {
	dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, "443"), &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(4 * time.Second))
	_, _ = io.WriteString(conn, fmt.Sprintf("HEAD / HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive\r\nUser-Agent: dialtone-watcher-test\r\n\r\n", host))

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	_ = conn.SetDeadline(time.Time{})
	select {
	case <-ctx.Done():
	case <-time.After(7 * time.Second):
	}
}
