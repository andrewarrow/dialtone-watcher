package watcher

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNormalizeHardwareLabel(t *testing.T) {
	got := normalizeHardwareLabel("Apple M4 Max")
	if got != "apple_m4_max" {
		t.Fatalf("normalizeHardwareLabel() = %q, want %q", got, "apple_m4_max")
	}
}

func TestUploaderBuildPayloadIncludesBoundedPeriodSummary(t *testing.T) {
	startedAt := time.Date(2026, time.March, 15, 10, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(30 * time.Second)

	u := &uploader{
		machineID: "machine-123",
		config: uploadConfig{
			Enabled:  true,
			Endpoint: "http://example.invalid",
			Interval: time.Minute,
			Timeout:  time.Second,
		},
		window: uploadWindow{
			startedAt:             startedAt,
			polls:                 6,
			maxTrackedProcesses:   12,
			maxTrackedDomains:     4,
			maxTrackedConnections: 5,
			totalRXBytes:          2048,
			totalTXBytes:          1024,
			processes: map[int32]*processUploadRecord{
				101: {
					PID:          101,
					Name:         "firefox",
					AverageCPU:   4.2,
					PeakCPU:      9.5,
					AverageRSSMB: 700,
					PeakRSSMB:    820,
					PollsSeen:    6,
				},
			},
			domains: map[string]*domainUploadRecord{
				"cloudflare.com": {
					Domain:    "cloudflare.com",
					RXBytes:   2048,
					TXBytes:   64,
					PollsSeen: 4,
				},
			},
			connections: map[string]*connectionUploadRecord{
				"101|HTTPS|cloudflare.com": {
					PID:         101,
					ProcessName: "firefox",
					Domain:      "cloudflare.com",
					Protocol:    "HTTPS",
					RXBytes:     2048,
					TXBytes:     64,
					PollsSeen:   4,
				},
			},
		},
	}

	hardware := HardwareProfile{
		Hostname:         "aas-MacBook-Pro.local",
		OS:               "darwin",
		Platform:         "macOS",
		CPUModel:         "Apple M4 Max",
		CPULogicalCores:  14,
		CPUPhysicalCores: 14,
		TotalMemoryGB:    36,
	}
	summary := Summary{
		Running:                true,
		TrackedProcessCount:    10,
		TrackedDomainCount:     4,
		TrackedConnectionCount: 5,
	}

	payload := u.buildPayload(endedAt, hardware, summary)

	if payload.Period.DurationSeconds != 30 {
		t.Fatalf("payload.Period.DurationSeconds = %d, want 30", payload.Period.DurationSeconds)
	}
	if payload.Summary.PollCount != 6 {
		t.Fatalf("payload.Summary.PollCount = %d, want 6", payload.Summary.PollCount)
	}
	if payload.Machine.Hardware.CPU.ModelNormalized != "apple_m4_max" {
		t.Fatalf("payload.Machine.Hardware.CPU.ModelNormalized = %q, want %q", payload.Machine.Hardware.CPU.ModelNormalized, "apple_m4_max")
	}
	if len(payload.Processes) != 1 || payload.Processes[0].Name != "firefox" {
		t.Fatalf("payload.Processes = %#v, want firefox entry", payload.Processes)
	}
	if payload.Summary.TotalRXBytes != 2048 || payload.Summary.TotalTXBytes != 1024 {
		t.Fatalf("payload summary bytes = (%d, %d), want (2048, 1024)", payload.Summary.TotalRXBytes, payload.Summary.TotalTXBytes)
	}
}

func TestUploaderSendPostsMachineIDHeaderAndJSONBody(t *testing.T) {
	var gotHeader string
	var gotPayload watcherUploadPayload

	u := &uploader{
		machineID: "machine-xyz",
		config: uploadConfig{
			Enabled:  true,
			Endpoint: "https://dialtoneapp.test/api/v1/watcher",
			Interval: time.Minute,
			Timeout:  time.Second,
		},
		client: &http.Client{
			Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				gotHeader = request.Header.Get("machine_id")
				if err := json.NewDecoder(request.Body).Decode(&gotPayload); err != nil {
					t.Fatalf("json decode error = %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusAccepted,
					Status:     "202 Accepted",
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}

	payload := watcherUploadPayload{
		SchemaVersion: 1,
		SentAt:        time.Date(2026, time.March, 15, 10, 5, 0, 0, time.UTC),
		Period: uploadPeriod{
			StartedAt:       time.Date(2026, time.March, 15, 10, 0, 0, 0, time.UTC),
			EndedAt:         time.Date(2026, time.March, 15, 10, 5, 0, 0, time.UTC),
			DurationSeconds: 300,
			Polls:           60,
		},
	}

	if err := u.send(payload); err != nil {
		t.Fatalf("send() error = %v", err)
	}
	if gotHeader != "machine-xyz" {
		t.Fatalf("machine_id header = %q, want %q", gotHeader, "machine-xyz")
	}
	if gotPayload.Period.Polls != 60 {
		t.Fatalf("payload.Period.Polls = %d, want 60", gotPayload.Period.Polls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
