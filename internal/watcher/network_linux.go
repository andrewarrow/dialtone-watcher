//go:build linux

package watcher

import (
	"fmt"

	gnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

func collectNetworkSamples() (map[string]networkObservation, error) {
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
			observations[key] = networkObservation{
				PID:      proc.Pid,
				Domain:   host,
				Protocol: inferSocketProtocol(connection.Type, connection.Raddr.Port),
				Sample:   networkConnectionSample{},
			}
		}
	}

	return observations, nil
}
