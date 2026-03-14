package watcher

import (
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
	Domain    string `json:"domain"`
	RXBytes   uint64 `json:"rx_bytes"`
	TXBytes   uint64 `json:"tx_bytes"`
	PollsSeen uint64 `json:"polls_seen"`
}

type Summary struct {
	PID                 int               `json:"pid"`
	Running             bool              `json:"running"`
	PollCount           uint64            `json:"poll_count"`
	TrackedProcessCount int               `json:"tracked_process_count"`
	Hardware            HardwareProfile   `json:"hardware"`
	TrackedDomainCount  int               `json:"tracked_domain_count"`
	TopProcess          ProcessSnapshot   `json:"top_process"`
	TopProcesses        []ProcessSnapshot `json:"top_processes"`
	TopDomains          []DomainSnapshot  `json:"top_domains"`
}

func LoadStatus() (Status, error) {
	var status Status
	err := readJSON(statusFilePath(), &status)
	return status, err
}

func LoadSummary() (Summary, error) {
	var summary Summary
	err := readJSON(summaryFilePath(), &summary)
	return summary, err
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

func ensureBaseDir() error {
	return os.MkdirAll(baseDir(), 0o755)
}

func writeJSON(path string, value any) error {
	if err := ensureBaseDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}
