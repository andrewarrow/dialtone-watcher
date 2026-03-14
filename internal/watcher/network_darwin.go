//go:build darwin

package watcher

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func collectNetworkSamples() (map[string]networkObservation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nettop", "-L", "1", "-x", "-J", "bytes_in,bytes_out,state")
	output, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("nettop failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}

	reader := csv.NewReader(strings.NewReader(string(output)))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	observations := make(map[string]networkObservation)
	var currentProcessID int32

	for {
		record, readErr := reader.Read()
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return nil, readErr
		}
		if len(record) == 0 {
			continue
		}

		first := strings.TrimSpace(record[0])
		if first == "" || first == "state" {
			continue
		}

		if pid, ok := parseProcessRow(first); ok {
			currentProcessID = pid
			continue
		}

		if currentProcessID == 0 {
			continue
		}

		domain, port, sample, ok := parseConnectionRow(first, record)
		if !ok {
			continue
		}

		key := fmt.Sprintf("%d|%s", currentProcessID, first)
		observations[key] = networkObservation{
			PID:      currentProcessID,
			Domain:   domain,
			Protocol: inferProtocol(port),
			Sample:   sample,
		}
	}

	return observations, nil
}

func parseProcessRow(value string) (int32, bool) {
	if strings.Contains(value, "<->") {
		return 0, false
	}

	idx := strings.LastIndex(value, ".")
	if idx == -1 || idx == len(value)-1 {
		return 0, false
	}

	pid, err := strconv.ParseInt(value[idx+1:], 10, 32)
	if err != nil {
		return 0, false
	}
	return int32(pid), true
}

func parseConnectionRow(spec string, record []string) (string, uint16, networkConnectionSample, bool) {
	var sample networkConnectionSample

	if len(record) < 4 || !strings.Contains(spec, "<->") {
		return "", 0, sample, false
	}

	parts := strings.SplitN(spec, " ", 2)
	if len(parts) != 2 {
		return "", 0, sample, false
	}
	endpoints := strings.SplitN(parts[1], "<->", 2)
	if len(endpoints) != 2 {
		return "", 0, sample, false
	}

	domain := normalizeRemoteHost(endpoints[1])
	if domain == "" {
		return "", 0, sample, false
	}

	port := parseEndpointPort(endpoints[1])

	sample.RXBytes = parseUint(record[2])
	sample.TXBytes = parseUint(record[3])

	return domain, port, sample, true
}

func normalizeRemoteHost(endpoint string) string {
	host := strings.TrimSpace(endpoint)
	if host == "" || host == "*:*" || host == "*.*" {
		return ""
	}

	if idx := strings.LastIndex(host, ":"); idx != -1 && isDigits(host[idx+1:]) {
		host = host[:idx]
	} else if idx := strings.LastIndex(host, "."); idx != -1 && isDigits(host[idx+1:]) && strings.Contains(host[:idx], ":") {
		host = host[:idx]
	}

	host = strings.Trim(host, "[]")
	if zoneIdx := strings.Index(host, "%"); zoneIdx != -1 {
		host = host[:zoneIdx]
	}
	host = strings.TrimSuffix(host, ".")
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || host == "*" || host == "localhost" {
		return ""
	}

	if addr, err := netip.ParseAddr(host); err == nil {
		if !isPublicIP(addr.String()) {
			return ""
		}
		return addr.String()
	}

	if isPublicIP(host) {
		return host
	}

	if strings.Count(host, ".") == 3 && strings.IndexFunc(host, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	}) == -1 {
		return ""
	}

	return host
}

func parseEndpointPort(endpoint string) uint16 {
	host := strings.TrimSpace(endpoint)
	if host == "" {
		return 0
	}

	if idx := strings.LastIndex(host, ":"); idx != -1 && isDigits(host[idx+1:]) {
		port, err := strconv.ParseUint(host[idx+1:], 10, 16)
		if err == nil {
			return uint16(port)
		}
	}

	if idx := strings.LastIndex(host, "."); idx != -1 && isDigits(host[idx+1:]) && strings.Contains(host[:idx], ":") {
		port, err := strconv.ParseUint(host[idx+1:], 10, 16)
		if err == nil {
			return uint16(port)
		}
	}

	return 0
}
