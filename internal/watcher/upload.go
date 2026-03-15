package watcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	defaultUploadURL          = "https://dialtoneapp.com/api/v1/watcher"
	defaultUploadInterval     = 15 * time.Minute
	defaultTestUploadInterval = 15 * time.Second
	defaultUploadTimeout      = 10 * time.Second
)

type uploadConfig struct {
	Enabled  bool
	Endpoint string
	Interval time.Duration
	Timeout  time.Duration
}

type uploader struct {
	config    uploadConfig
	machineID string
	client    *http.Client
	window    uploadWindow
}

type uploadWindow struct {
	startedAt             time.Time
	polls                 uint64
	maxTrackedProcesses   int
	maxTrackedDomains     int
	maxTrackedConnections int
	totalRXBytes          uint64
	totalTXBytes          uint64
	processes             map[int32]*processUploadRecord
	domains               map[string]*domainUploadRecord
	connections           map[string]*connectionUploadRecord
}

type processUploadRecord struct {
	PID            int32   `json:"pid"`
	Name           string  `json:"name"`
	Command        string  `json:"command,omitempty"`
	AverageCPU     float64 `json:"average_cpu_percent"`
	PeakCPU        float64 `json:"peak_cpu_percent"`
	AverageRSSMB   float64 `json:"average_memory_rss_mb"`
	PeakRSSMB      float64 `json:"peak_memory_rss_mb"`
	PollsSeen      uint64  `json:"polls_seen"`
	CommandPresent bool    `json:"-"`

	cpuTotal float64
	rssTotal float64
}

type domainUploadRecord struct {
	Domain      string `json:"domain"`
	DisplayName string `json:"display_name,omitempty"`
	RXBytes     uint64 `json:"rx_bytes"`
	TXBytes     uint64 `json:"tx_bytes"`
	PollsSeen   uint64 `json:"polls_seen"`
}

type connectionUploadRecord struct {
	PID         int32  `json:"pid"`
	ProcessName string `json:"process_name,omitempty"`
	Domain      string `json:"domain"`
	DisplayName string `json:"display_name,omitempty"`
	Protocol    string `json:"protocol"`
	RXBytes     uint64 `json:"rx_bytes"`
	TXBytes     uint64 `json:"tx_bytes"`
	PollsSeen   uint64 `json:"polls_seen"`
}

type watcherUploadPayload struct {
	SchemaVersion int                      `json:"schema_version"`
	SentAt        time.Time                `json:"sent_at"`
	Period        uploadPeriod             `json:"period"`
	Machine       uploadMachine            `json:"machine"`
	Summary       uploadSummary            `json:"summary"`
	Processes     []processUploadRecord    `json:"processes"`
	Domains       []domainUploadRecord     `json:"domains"`
	Connections   []connectionUploadRecord `json:"connections"`
}

type uploadPeriod struct {
	StartedAt       time.Time `json:"started_at"`
	EndedAt         time.Time `json:"ended_at"`
	DurationSeconds int64     `json:"duration_seconds"`
	Polls           uint64    `json:"polls"`
}

type uploadMachine struct {
	Hostname        string         `json:"hostname"`
	OS              string         `json:"os"`
	Platform        string         `json:"platform"`
	PlatformVersion string         `json:"platform_version"`
	KernelVersion   string         `json:"kernel_version"`
	Hardware        uploadHardware `json:"hardware"`
}

type uploadHardware struct {
	CPU    uploadCPU    `json:"cpu"`
	Memory uploadMemory `json:"memory"`
	Disk   uploadDisk   `json:"disk"`
}

type uploadCPU struct {
	Model           string  `json:"model"`
	ModelNormalized string  `json:"model_normalized"`
	PhysicalCores   int     `json:"physical_cores"`
	LogicalCores    int     `json:"logical_cores"`
	FrequencyMHz    float64 `json:"frequency_mhz"`
}

type uploadMemory struct {
	TotalGB float64 `json:"total_gb"`
}

type uploadDisk struct {
	TotalGB float64 `json:"total_gb"`
}

type uploadSummary struct {
	Running                bool   `json:"running"`
	PollCount              uint64 `json:"poll_count"`
	TrackedProcessCount    int    `json:"tracked_process_count"`
	TrackedDomainCount     int    `json:"tracked_domain_count"`
	TrackedConnectionCount int    `json:"tracked_connection_count"`
	TotalRXBytes           uint64 `json:"total_rx_bytes"`
	TotalTXBytes           uint64 `json:"total_tx_bytes"`
}

func newUploader(machineID string, options RunOptions) *uploader {
	config := loadUploadConfig(options)
	if !config.Enabled || config.Endpoint == "" || machineID == "" {
		return nil
	}

	return &uploader{
		config:    config,
		machineID: machineID,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

func loadUploadConfig(options RunOptions) uploadConfig {
	config := uploadConfig{
		Enabled:  true,
		Endpoint: defaultUploadURL,
		Interval: defaultUploadInterval,
		Timeout:  defaultUploadTimeout,
	}

	if options.TestMode {
		config.Interval = defaultTestUploadInterval
	}

	if value := strings.TrimSpace(os.Getenv("DIALTONE_WATCHER_UPLOAD_URL")); value != "" {
		config.Endpoint = value
	}
	if value := strings.TrimSpace(os.Getenv("DIALTONE_WATCHER_UPLOAD_INTERVAL")); value != "" {
		if duration, err := time.ParseDuration(value); err == nil && duration > 0 {
			config.Interval = duration
		}
	}
	if value := strings.TrimSpace(os.Getenv("DIALTONE_WATCHER_UPLOAD_TIMEOUT")); value != "" {
		if duration, err := time.ParseDuration(value); err == nil && duration > 0 {
			config.Timeout = duration
		}
	}
	if disabled := strings.TrimSpace(strings.ToLower(os.Getenv("DIALTONE_WATCHER_DISABLE_UPLOAD"))); disabled == "1" || disabled == "true" || disabled == "yes" {
		config.Enabled = false
	}

	return config
}

func (u *uploader) recordProcesses(now time.Time, pids map[int32]*processRecord) {
	if u == nil {
		return
	}

	u.ensureWindow(now)
	for pid, record := range pids {
		if record == nil {
			continue
		}

		current := u.window.processes[pid]
		if current == nil {
			current = &processUploadRecord{
				PID:  record.PID,
				Name: record.Name,
			}
			u.window.processes[pid] = current
		}

		current.Name = record.Name
		if record.Command != "" {
			current.Command = record.Command
			current.CommandPresent = true
		}
		current.cpuTotal += record.CPUPercent
		current.rssTotal += record.MemoryRSSMB
		current.PollsSeen++
		current.AverageCPU = current.cpuTotal / float64(current.PollsSeen)
		current.AverageRSSMB = current.rssTotal / float64(current.PollsSeen)
		if record.CPUPercent > current.PeakCPU {
			current.PeakCPU = record.CPUPercent
		}
		if record.MemoryRSSMB > current.PeakRSSMB {
			current.PeakRSSMB = record.MemoryRSSMB
		}
	}

	if len(pids) > u.window.maxTrackedProcesses {
		u.window.maxTrackedProcesses = len(pids)
	}
}

func (u *uploader) recordNetwork(now time.Time, sample networkObservation, rxBytes, txBytes uint64, trackedDomains, trackedConnections int) {
	if u == nil {
		return
	}

	u.ensureWindow(now)
	u.window.totalRXBytes += rxBytes
	u.window.totalTXBytes += txBytes

	domain := u.window.domains[sample.Domain]
	if domain == nil {
		domain = &domainUploadRecord{Domain: sample.Domain}
		u.window.domains[sample.Domain] = domain
	}
	domain.DisplayName = sample.DisplayName
	domain.RXBytes += rxBytes
	domain.TXBytes += txBytes
	domain.PollsSeen++

	key := observedConnectionKey(sample)
	connection := u.window.connections[key]
	if connection == nil {
		connection = &connectionUploadRecord{
			PID:      sample.PID,
			Domain:   sample.Domain,
			Protocol: sample.Protocol,
		}
		u.window.connections[key] = connection
	}
	connection.ProcessName = sample.ProcessName
	connection.DisplayName = sample.DisplayName
	connection.RXBytes += rxBytes
	connection.TXBytes += txBytes
	connection.PollsSeen++

	if trackedDomains > u.window.maxTrackedDomains {
		u.window.maxTrackedDomains = trackedDomains
	}
	if trackedConnections > u.window.maxTrackedConnections {
		u.window.maxTrackedConnections = trackedConnections
	}
}

func (u *uploader) incrementPoll(now time.Time) {
	if u == nil {
		return
	}
	u.ensureWindow(now)
	u.window.polls++
}

func (u *uploader) shouldUpload(now time.Time) bool {
	if u == nil {
		return false
	}
	if u.window.startedAt.IsZero() || u.window.polls == 0 {
		return false
	}
	return now.Sub(u.window.startedAt) >= u.config.Interval
}

func (u *uploader) flush(now time.Time, hardware HardwareProfile, summary Summary) error {
	if u == nil {
		return nil
	}
	if u.window.startedAt.IsZero() || u.window.polls == 0 {
		return nil
	}

	payload := u.buildPayload(now, hardware, summary)
	if err := u.send(payload); err != nil {
		return err
	}
	u.resetWindow(now)
	return nil
}

func (u *uploader) ensureWindow(now time.Time) {
	if !u.window.startedAt.IsZero() {
		return
	}

	u.window = uploadWindow{
		startedAt:   now,
		processes:   make(map[int32]*processUploadRecord),
		domains:     make(map[string]*domainUploadRecord),
		connections: make(map[string]*connectionUploadRecord),
	}
}

func (u *uploader) resetWindow(now time.Time) {
	u.window = uploadWindow{
		startedAt:   now,
		processes:   make(map[int32]*processUploadRecord),
		domains:     make(map[string]*domainUploadRecord),
		connections: make(map[string]*connectionUploadRecord),
	}
}

func (u *uploader) buildPayload(now time.Time, hardware HardwareProfile, summary Summary) watcherUploadPayload {
	return watcherUploadPayload{
		SchemaVersion: 1,
		SentAt:        now.UTC(),
		Period: uploadPeriod{
			StartedAt:       u.window.startedAt.UTC(),
			EndedAt:         now.UTC(),
			DurationSeconds: int64(now.Sub(u.window.startedAt) / time.Second),
			Polls:           u.window.polls,
		},
		Machine: uploadMachine{
			Hostname:        hardware.Hostname,
			OS:              hardware.OS,
			Platform:        hardware.Platform,
			PlatformVersion: hardware.PlatformVersion,
			KernelVersion:   hardware.KernelVersion,
			Hardware:        normalizeHardware(hardware),
		},
		Summary: uploadSummary{
			Running:                summary.Running,
			PollCount:              u.window.polls,
			TrackedProcessCount:    max(summary.TrackedProcessCount, u.window.maxTrackedProcesses),
			TrackedDomainCount:     max(summary.TrackedDomainCount, u.window.maxTrackedDomains),
			TrackedConnectionCount: max(summary.TrackedConnectionCount, u.window.maxTrackedConnections),
			TotalRXBytes:           u.window.totalRXBytes,
			TotalTXBytes:           u.window.totalTXBytes,
		},
		Processes:   allUploadProcesses(u.window.processes),
		Domains:     allUploadDomains(u.window.domains),
		Connections: allUploadConnections(u.window.connections),
	}
}

func (u *uploader) send(payload watcherUploadPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, u.config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("machine_id", u.machineID)

	response, err := u.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("watcher upload failed: status %s", response.Status)
	}

	return nil
}

func normalizeHardware(hardware HardwareProfile) uploadHardware {
	return uploadHardware{
		CPU: uploadCPU{
			Model:           hardware.CPUModel,
			ModelNormalized: normalizeHardwareLabel(hardware.CPUModel),
			PhysicalCores:   hardware.CPUPhysicalCores,
			LogicalCores:    hardware.CPULogicalCores,
			FrequencyMHz:    hardware.CPUFrequencyMHz,
		},
		Memory: uploadMemory{TotalGB: hardware.TotalMemoryGB},
		Disk:   uploadDisk{TotalGB: hardware.TotalDiskGB},
	}
}

var hardwareLabelPattern = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeHardwareLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	value = hardwareLabelPattern.ReplaceAllString(value, "_")
	return strings.Trim(value, "_")
}

func allUploadProcesses(records map[int32]*processUploadRecord) []processUploadRecord {
	if len(records) == 0 {
		return nil
	}

	items := make([]processUploadRecord, 0, len(records))
	for _, record := range records {
		if record == nil || record.Name == "" || record.PollsSeen == 0 {
			continue
		}
		copy := *record
		if !copy.CommandPresent {
			copy.Command = ""
		}
		items = append(items, copy)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].AverageCPU == items[j].AverageCPU {
			if items[i].AverageRSSMB == items[j].AverageRSSMB {
				return items[i].PollsSeen > items[j].PollsSeen
			}
			return items[i].AverageRSSMB > items[j].AverageRSSMB
		}
		return items[i].AverageCPU > items[j].AverageCPU
	})

	return items
}

func allUploadDomains(records map[string]*domainUploadRecord) []domainUploadRecord {
	if len(records) == 0 {
		return nil
	}

	items := make([]domainUploadRecord, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		items = append(items, *record)
	}

	sort.Slice(items, func(i, j int) bool {
		left := items[i].RXBytes + items[i].TXBytes
		right := items[j].RXBytes + items[j].TXBytes
		if left == right {
			return items[i].PollsSeen > items[j].PollsSeen
		}
		return left > right
	})

	return items
}

func allUploadConnections(records map[string]*connectionUploadRecord) []connectionUploadRecord {
	if len(records) == 0 {
		return nil
	}

	items := make([]connectionUploadRecord, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		items = append(items, *record)
	}

	sort.Slice(items, func(i, j int) bool {
		left := items[i].RXBytes + items[i].TXBytes
		right := items[j].RXBytes + items[j].TXBytes
		if left == right {
			return items[i].PollsSeen > items[j].PollsSeen
		}
		return left > right
	})

	return items
}
