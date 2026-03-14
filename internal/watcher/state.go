package watcher

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

type Status struct {
	PID int `json:"pid"`
}

type ProcessSnapshot struct {
	PID         int32   `json:"pid"`
	Name        string  `json:"name"`
	Command     string  `json:"command"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryRSSMB float64 `json:"memory_rss_mb"`
	PollsSeen   uint64  `json:"polls_seen"`
}

type DomainSnapshot struct {
	Domain      string `json:"domain"`
	DisplayName string `json:"display_name,omitempty"`
	RXBytes     uint64 `json:"rx_bytes"`
	TXBytes     uint64 `json:"tx_bytes"`
	PollsSeen   uint64 `json:"polls_seen"`
}

type ConnectionSnapshot struct {
	PID         int32  `json:"pid"`
	ProcessName string `json:"process_name,omitempty"`
	Domain      string `json:"domain"`
	DisplayName string `json:"display_name,omitempty"`
	Protocol    string `json:"protocol"`
	RXBytes     uint64 `json:"rx_bytes"`
	TXBytes     uint64 `json:"tx_bytes"`
	PollsSeen   uint64 `json:"polls_seen"`
}

type Summary struct {
	PID                    int                  `json:"pid"`
	Running                bool                 `json:"running"`
	PollCount              uint64               `json:"poll_count"`
	TrackedProcessCount    int                  `json:"tracked_process_count"`
	Hardware               HardwareProfile      `json:"hardware"`
	TrackedDomainCount     int                  `json:"tracked_domain_count"`
	TrackedConnectionCount int                  `json:"tracked_connection_count"`
	TopProcess             ProcessSnapshot      `json:"top_process"`
	TopProcesses           []ProcessSnapshot    `json:"top_processes"`
	TopDomains             []DomainSnapshot     `json:"top_domains"`
	TopConnections         []ConnectionSnapshot `json:"top_connections"`
}

type machineIdentityRecord struct {
	MachineID string `json:"machine_id"`
}

func LoadStatus() (Status, error) {
	var status Status
	err := readJSON(statusFilePath(), &status)
	return status, err
}

func LoadSummary() (Summary, error) {
	var summary Summary
	err := readJSON(summaryFilePath(), &summary)
	if err != nil {
		return summary, err
	}

	if !summary.Running {
		summary.PID = 0
		return summary, nil
	}

	if summary.PID == 0 || !IsProcessRunning(summary.PID) {
		summary.Running = false
		summary.PID = 0
	}

	return summary, nil
}

func writeStatus(status Status) error {
	return writeJSON(statusFilePath(), status)
}

func writeSummary(summary Summary) error {
	return writeJSON(summaryFilePath(), summary)
}

func removeStatus() error {
	err := os.Remove(statusFilePath())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func baseDir() string {
	if override := os.Getenv("DIALTONE_WATCHER_HOME"); override != "" {
		return override
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "dialtone-watcher")
	}
	return filepath.Join(cacheDir, "dialtone-watcher")
}

func statusFilePath() string {
	return filepath.Join(baseDir(), "status.json")
}

func summaryFilePath() string {
	return filepath.Join(baseDir(), "summary.json")
}

func machineIdentityFilePath() string {
	return filepath.Join(baseDir(), "machine-id.json")
}

func ensureBaseDir() error {
	return os.MkdirAll(baseDir(), 0o755)
}

func MachineID() (string, error) {
	record, err := loadOrCreateMachineIdentity()
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256([]byte(record.MachineID + "|dialtone-watcher"))
	return hex.EncodeToString(sum[:]), nil
}

func loadOrCreateMachineIdentity() (machineIdentityRecord, error) {
	var record machineIdentityRecord
	path := machineIdentityFilePath()

	if err := readJSON(path, &record); err == nil {
		if record.MachineID != "" {
			return record, nil
		}
	} else if !os.IsNotExist(err) {
		return record, err
	}

	machineID, err := newMachineIdentity()
	if err != nil {
		return machineIdentityRecord{}, err
	}
	record.MachineID = machineID
	if err := writeJSONFile(path, record, 0o600); err != nil {
		return machineIdentityRecord{}, err
	}

	return record, nil
}

func newMachineIdentity() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func writeJSON(path string, value any) error {
	return writeJSONFile(path, value, 0o644)
}

func writeJSONFile(path string, value any, permissions os.FileMode) error {
	if err := ensureBaseDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, permissions)
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}
