//go:build linux

package watcher

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	gnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

func collectNetworkSamples() (map[string]networkObservation, error) {
	tcpByteSamples := collectLinuxSocketByteSamples()
	udpByteSamples := collectLinuxUDPFlowSamples()

	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	observations := make(map[string]networkObservation)

	for _, proc := range processes {
		connections, err := gnet.ConnectionsPid("inet", proc.Pid)
		if err != nil {
			continue
		}

		for _, connection := range connections {
			host := normalizeResolvedHost(connection.Raddr.IP)
			if host == "" {
				continue
			}

			key := fmt.Sprintf(
				"%d|%d|%d|%s|%d|%s|%d",
				proc.Pid,
				connection.Fd,
				connection.Type,
				connection.Laddr.IP,
				connection.Laddr.Port,
				host,
				connection.Raddr.Port,
			)
			sample := tcpByteSamples[key]
			if connection.Type == syscall.SOCK_DGRAM {
				sample = udpByteSamples[linuxFlowTupleKey(
					connection.Type,
					connection.Laddr.IP,
					connection.Laddr.Port,
					host,
					connection.Raddr.Port,
				)]
			}
			observations[key] = networkObservation{
				PID:      proc.Pid,
				Domain:   host,
				Protocol: inferSocketProtocol(connection.Type, connection.Raddr.Port),
				Sample:   sample,
			}
		}
	}

	return observations, nil
}

func collectLinuxSocketByteSamples() map[string]networkConnectionSample {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ss", "-tinpHOn")
	output, err := cmd.Output()
	if err != nil || ctx.Err() != nil {
		return map[string]networkConnectionSample{}
	}

	return parseLinuxSocketByteSamples(string(output))
}

func collectLinuxUDPFlowSamples() map[string]networkConnectionSample {
	samples := make(map[string]networkConnectionSample)
	for _, path := range []string{"/proc/net/nf_conntrack", "/proc/net/ip_conntrack"} {
		file, err := os.Open(path)
		if err != nil {
			continue
		}

		parsed := parseLinuxConntrackSamples(file)
		_ = file.Close()
		if len(parsed) == 0 {
			continue
		}
		for key, sample := range parsed {
			samples[key] = sample
		}
		return samples
	}
	return samples
}

func parseLinuxSocketByteSamples(raw string) map[string]networkConnectionSample {
	samples := make(map[string]networkConnectionSample)

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		localHost, localPort, ok := parseLinuxSocketEndpoint(fields[3])
		if !ok {
			continue
		}

		remoteHost, remotePort, ok := parseLinuxSocketEndpoint(fields[4])
		if !ok {
			continue
		}

		host := normalizeResolvedHost(remoteHost)
		if host == "" {
			continue
		}

		sample := networkConnectionSample{
			RXBytes: findLinuxSocketCounter(line, "bytes_received:"),
			TXBytes: findLinuxSocketCounter(line, "bytes_acked:"),
		}
		if sample.RXBytes == 0 && sample.TXBytes == 0 {
			continue
		}

		for _, user := range parseLinuxSocketUsers(line) {
			key := fmt.Sprintf(
				"%d|%d|%d|%s|%d|%s|%d",
				user.pid,
				user.fd,
				user.socketType,
				localHost,
				localPort,
				host,
				remotePort,
			)
			samples[key] = sample
		}
	}

	return samples
}

type linuxSocketUser struct {
	pid        int32
	fd         uint32
	socketType uint32
}

func parseLinuxSocketUsers(line string) []linuxSocketUser {
	users := make([]linuxSocketUser, 0, 1)

	marker := "users:("
	start := strings.Index(line, marker)
	if start == -1 {
		return users
	}

	segment := line[start+len(marker):]
	if end := strings.Index(segment, ") "); end != -1 {
		segment = segment[:end]
	}

	for _, part := range strings.Split(segment, "pid=") {
		if part == "" {
			continue
		}

		pidEnd := strings.IndexByte(part, ',')
		if pidEnd == -1 {
			continue
		}
		pidValue, err := strconv.ParseInt(part[:pidEnd], 10, 32)
		if err != nil {
			continue
		}

		fdIndex := strings.Index(part[pidEnd+1:], "fd=")
		if fdIndex == -1 {
			continue
		}
		fdPart := part[pidEnd+1+fdIndex+len("fd="):]
		fdEnd := strings.IndexAny(fdPart, "),")
		if fdEnd == -1 {
			fdEnd = len(fdPart)
		}
		fdValue, err := strconv.ParseUint(fdPart[:fdEnd], 10, 32)
		if err != nil {
			continue
		}

		users = append(users, linuxSocketUser{
			pid:        int32(pidValue),
			fd:         uint32(fdValue),
			socketType: syscall.SOCK_STREAM,
		})
	}

	return users
}

func findLinuxSocketCounter(line, prefix string) uint64 {
	start := strings.Index(line, prefix)
	if start == -1 {
		return 0
	}

	value := line[start+len(prefix):]
	end := strings.IndexByte(value, ' ')
	if end != -1 {
		value = value[:end]
	}

	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseLinuxSocketEndpoint(value string) (string, uint32, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "*:*" {
		return "", 0, false
	}

	idx := strings.LastIndex(value, ":")
	if idx == -1 || idx == len(value)-1 || !isDigits(value[idx+1:]) {
		return "", 0, false
	}

	port, err := strconv.ParseUint(value[idx+1:], 10, 32)
	if err != nil {
		return "", 0, false
	}

	host := strings.TrimSpace(value[:idx])
	host = strings.Trim(host, "[]")
	if zoneIdx := strings.Index(host, "%"); zoneIdx != -1 {
		host = host[:zoneIdx]
	}

	if host == "" || host == "*" {
		return "", 0, false
	}

	return host, uint32(port), true
}

func parseLinuxConntrackSamples(reader io.Reader) map[string]networkConnectionSample {
	samples := make(map[string]networkConnectionSample)
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1024*1024)

	for scanner.Scan() {
		recordSamples := parseLinuxConntrackLine(scanner.Text())
		for key, sample := range recordSamples {
			samples[key] = sample
		}
	}

	return samples
}

func parseLinuxConntrackLine(line string) map[string]networkConnectionSample {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 10 {
		return nil
	}

	protocol := fields[0]
	offset := 0
	if protocol == "ipv4" || protocol == "ipv6" {
		if len(fields) < 11 {
			return nil
		}
		protocol = fields[2]
		offset = 2
	}

	if protocol != "udp" {
		return nil
	}

	attrs := make(map[string][]string)
	for _, field := range fields[offset+3:] {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		attrs[key] = append(attrs[key], value)
	}

	origSrc, origDst, origSport, origDport, origBytes, ok := parseLinuxConntrackTuple(attrs, 0)
	if !ok {
		return nil
	}
	replySrc, replyDst, replySport, replyDport, replyBytes, ok := parseLinuxConntrackTuple(attrs, 1)
	if !ok {
		return nil
	}

	return map[string]networkConnectionSample{
		linuxFlowTupleKey(syscall.SOCK_DGRAM, origSrc, origSport, origDst, origDport): {
			RXBytes: replyBytes,
			TXBytes: origBytes,
		},
		linuxFlowTupleKey(syscall.SOCK_DGRAM, replySrc, replySport, replyDst, replyDport): {
			RXBytes: origBytes,
			TXBytes: replyBytes,
		},
	}
}

func parseLinuxConntrackTuple(attrs map[string][]string, index int) (string, string, uint32, uint32, uint64, bool) {
	src := linuxConntrackAttr(attrs, "src", index)
	dst := linuxConntrackAttr(attrs, "dst", index)
	sportValue := linuxConntrackAttr(attrs, "sport", index)
	dportValue := linuxConntrackAttr(attrs, "dport", index)
	bytesValue := linuxConntrackAttr(attrs, "bytes", index)
	if src == "" || dst == "" || sportValue == "" || dportValue == "" || bytesValue == "" {
		return "", "", 0, 0, 0, false
	}

	src = normalizeLinuxConntrackHost(src)
	dst = normalizeLinuxConntrackHost(dst)
	if src == "" || dst == "" {
		return "", "", 0, 0, 0, false
	}

	sport, err := strconv.ParseUint(sportValue, 10, 32)
	if err != nil {
		return "", "", 0, 0, 0, false
	}
	dport, err := strconv.ParseUint(dportValue, 10, 32)
	if err != nil {
		return "", "", 0, 0, 0, false
	}
	bytesCount, err := strconv.ParseUint(bytesValue, 10, 64)
	if err != nil {
		return "", "", 0, 0, 0, false
	}

	return src, dst, uint32(sport), uint32(dport), bytesCount, true
}

func linuxConntrackAttr(attrs map[string][]string, key string, index int) string {
	values := attrs[key]
	if len(values) <= index {
		return ""
	}
	return values[index]
}

func linuxFlowTupleKey(socketType uint32, localHost string, localPort uint32, remoteHost string, remotePort uint32) string {
	return fmt.Sprintf("%d|%s|%d|%s|%d", socketType, localHost, localPort, remoteHost, remotePort)
}

func normalizeLinuxConntrackHost(host string) string {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return ""
	}
	if zoneIdx := strings.Index(host, "%"); zoneIdx != -1 {
		host = host[:zoneIdx]
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.String()
	}
	return strings.ToLower(host)
}
