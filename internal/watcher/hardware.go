package watcher

import (
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
	TotalDiskGB   float64 `json:"total_disk_gb"`
}

func CollectHardwareProfile() (HardwareProfile, error) {
	var profile HardwareProfile

	hostInfo, err := host.Info()
	if err != nil {
		return profile, err
	}

	profile.Hostname = hostInfo.Hostname
	profile.OS = hostInfo.OS
	profile.Platform = hostInfo.Platform
	profile.PlatformVersion = hostInfo.PlatformVersion
	profile.KernelVersion = hostInfo.KernelVersion
	profile.UptimeSeconds = hostInfo.Uptime

	cpuInfo, err := cpu.Info()
	if err == nil && len(cpuInfo) > 0 {
		profile.CPUModel = cpuInfo[0].ModelName
		profile.CPUFrequencyMHz = cpuInfo[0].Mhz
	}

	if physical, err := cpu.Counts(false); err == nil {
		profile.CPUPhysicalCores = physical
	}
	if logical, err := cpu.Counts(true); err == nil {
		profile.CPULogicalCores = logical
	}

	if memInfo, err := mem.VirtualMemory(); err == nil {
		profile.TotalMemoryGB = float64(memInfo.Total) / (1024 * 1024 * 1024)
	}

	partitions, err := disk.Partitions(false)
	if err == nil {
		var totalDisk uint64
		for _, partition := range partitions {
			usage, usageErr := disk.Usage(partition.Mountpoint)
			if usageErr == nil {
				totalDisk += usage.Total
			}
		}
		profile.TotalDiskGB = float64(totalDisk) / (1024 * 1024 * 1024)
	}

	return profile, nil
}
