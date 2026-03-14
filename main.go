package main

import (
	"encoding/json"
	"fmt"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

type HardwareProfile struct {
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	KernelVersion   string `json:"kernel_version"`
	UptimeSeconds   uint64 `json:"uptime_seconds"`

	CPUModel         string  `json:"cpu_model"`
	CPUPhysicalCores int     `json:"cpu_physical_cores"`
	CPULogicalCores  int     `json:"cpu_logical_cores"`
	CPUFrequencyMHz  float64 `json:"cpu_frequency_mhz"`

	TotalMemoryGB float64 `json:"total_memory_gb"`

	TotalDiskGB float64 `json:"total_disk_gb"`
}

func main() {

	var profile HardwareProfile

	// host info
	hostInfo, _ := host.Info()

	profile.Hostname = hostInfo.Hostname
	profile.OS = hostInfo.OS
	profile.Platform = hostInfo.Platform
	profile.PlatformVersion = hostInfo.PlatformVersion
	profile.KernelVersion = hostInfo.KernelVersion
	profile.UptimeSeconds = hostInfo.Uptime

	// cpu info
	cpuInfo, _ := cpu.Info()
	if len(cpuInfo) > 0 {
		profile.CPUModel = cpuInfo[0].ModelName
		profile.CPUFrequencyMHz = cpuInfo[0].Mhz
	}

	physical, _ := cpu.Counts(false)
	logical, _ := cpu.Counts(true)

	profile.CPUPhysicalCores = physical
	profile.CPULogicalCores = logical

	// memory
	memInfo, _ := mem.VirtualMemory()
	profile.TotalMemoryGB = float64(memInfo.Total) / (1024 * 1024 * 1024)

	// disk
	partitions, _ := disk.Partitions(false)

	var totalDisk uint64

	for _, p := range partitions {
		usage, err := disk.Usage(p.Mountpoint)
		if err == nil {
			totalDisk += usage.Total
		}
	}

	profile.TotalDiskGB = float64(totalDisk) / (1024 * 1024 * 1024)

	// print json
	jsonData, _ := json.MarshalIndent(profile, "", "  ")
	fmt.Println(string(jsonData))
}
