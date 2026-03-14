package watcher

import (
	"strconv"
	"strings"
	"syscall"
)

type networkConnectionSample struct {
	RXBytes uint64
	TXBytes uint64
}

type networkObservation struct {
	PID      int32
	Domain   string
	Protocol string
	Sample   networkConnectionSample
}

func inferProtocol(port uint16) string {
	switch port {
	case 22:
		return "SSH"
	case 53:
		return "DNS"
	case 80:
		return "HTTP"
	case 443:
		return "HTTPS"
	case 853:
		return "DNS-over-TLS"
	case 3306:
		return "MySQL"
	case 5432:
		return "Postgres"
	case 6379:
		return "Redis"
	case 784, 4433:
		return "QUIC"
	default:
		return "TCP"
	}
}

func inferSocketProtocol(socketType uint32, port uint32) string {
	if socketType == syscall.SOCK_DGRAM {
		switch port {
		case 53:
			return "DNS"
		case 443, 784, 4433:
			return "QUIC"
		default:
			return "UDP"
		}
	}

	return inferProtocol(uint16(port))
}

func parseUint(value string) uint64 {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
