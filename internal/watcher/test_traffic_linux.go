//go:build linux

package watcher

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

type httpTarget struct {
	Host string
	Path string
	TLS  bool
}

var linuxHTTPTestTargets = []httpTarget{
	{Host: "google.com", Path: "/", TLS: true},
	{Host: "apple.com", Path: "/", TLS: true},
	{Host: "microsoft.com", Path: "/", TLS: true},
	{Host: "example.com", Path: "/", TLS: false},
	{Host: "neverssl.com", Path: "/", TLS: false},
}

var linuxDNSTestTargets = []string{
	"google.com",
	"cloudflare.com",
	"github.com",
	"openai.com",
}

var linuxDNSServers = []string{
	"1.1.1.1:53",
	"8.8.8.8:53",
}

func startTestTraffic(enabled bool) func() {
	if !enabled {
		return func() {}
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	for _, target := range linuxHTTPTestTargets {
		wg.Add(1)
		go func(target httpTarget) {
			defer wg.Done()
			runHTTPTestLoop(ctx, target)
		}(target)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		runLookupLoop(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		runUDPDNSLoop(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		runCPULoadLoop(ctx)
	}()

	return func() {
		cancel()
		wg.Wait()
	}
}

func runHTTPTestLoop(ctx context.Context, target httpTarget) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		driveHTTPConnection(ctx, target)

		timer.Reset(1500 * time.Millisecond)
	}
}

func driveHTTPConnection(ctx context.Context, target httpTarget) {
	dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	address := net.JoinHostPort(target.Host, portForTarget(target))

	var conn net.Conn
	var err error
	if target.TLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, &tls.Config{
			ServerName: target.Host,
			MinVersion: tls.VersionTLS12,
		})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(4 * time.Second))
	_, _ = io.WriteString(conn, fmt.Sprintf(
		"GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\nUser-Agent: dialtone-watcher-test\r\nAccept: */*\r\n\r\n",
		target.Path,
		target.Host,
	))

	reader := bufio.NewReader(conn)
	_, _ = reader.ReadString('\n')
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	buffer := make([]byte, 2048)
	_, _ = io.ReadFull(reader, buffer)
	_ = conn.SetDeadline(time.Time{})

	select {
	case <-ctx.Done():
	case <-time.After(2500 * time.Millisecond):
	}
}

func portForTarget(target httpTarget) string {
	if target.TLS {
		return "443"
	}
	return "80"
}

func runLookupLoop(ctx context.Context) {
	resolver := net.Resolver{}
	timer := time.NewTimer(0)
	defer timer.Stop()

	index := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		host := linuxDNSTestTargets[index%len(linuxDNSTestTargets)]
		index++

		lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, _ = resolver.LookupIPAddr(lookupCtx, host)
		cancel()

		timer.Reset(1200 * time.Millisecond)
	}
}

func runUDPDNSLoop(ctx context.Context) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	serverIndex := 0
	nameIndex := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		server := linuxDNSServers[serverIndex%len(linuxDNSServers)]
		host := linuxDNSTestTargets[nameIndex%len(linuxDNSTestTargets)]
		serverIndex++
		nameIndex++

		sendUDPDNSQuery(ctx, server, host)
		timer.Reset(1800 * time.Millisecond)
	}
}

func sendUDPDNSQuery(ctx context.Context, server, host string) {
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "udp", server)
	if err != nil {
		return
	}
	defer conn.Close()

	query := buildDNSQuery(host)
	if len(query) == 0 {
		return
	}

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = conn.Write(query)

	buffer := make([]byte, 512)
	_, _ = conn.Read(buffer)
}

func buildDNSQuery(host string) []byte {
	var question bytes.Buffer
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return nil
		}
		question.WriteByte(byte(len(label)))
		question.WriteString(label)
	}
	question.WriteByte(0)

	packet := make([]byte, 12)
	binary.BigEndian.PutUint16(packet[0:2], uint16(rand.Intn(65535)))
	binary.BigEndian.PutUint16(packet[2:4], 0x0100)
	binary.BigEndian.PutUint16(packet[4:6], 1)

	packet = append(packet, question.Bytes()...)
	packet = append(packet, 0, 1)
	packet = append(packet, 0, 1)
	return packet
}

func runCPULoadLoop(ctx context.Context) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		runCPUBurst(ctx, 900*time.Millisecond)
		timer.Reset(3 * time.Second)
	}
}

func runCPUBurst(ctx context.Context, duration time.Duration) {
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		payload := bytes.Repeat([]byte("dialtone-watcher-linux-test-traffic-"), 4096)
		payload = append(payload, byte(rand.Intn(255)))

		sum := sha256.Sum256(payload)
		var compressed bytes.Buffer
		writer, err := gzip.NewWriterLevel(&compressed, gzip.BestSpeed)
		if err != nil {
			return
		}
		_, _ = writer.Write(sum[:])
		_, _ = writer.Write(payload[:32768])
		_ = writer.Close()

		second := sha256.Sum256(compressed.Bytes())
		if second[0] == 255 {
			time.Sleep(10 * time.Millisecond)
		}
	}
}
