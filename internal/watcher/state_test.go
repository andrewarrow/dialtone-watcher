package watcher

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestMachineIDPersistsAndUsesStoredUUIDValue(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DIALTONE_WATCHER_HOME", tempDir)

	first, err := MachineID()
	if err != nil {
		t.Fatalf("MachineID() first call error = %v", err)
	}

	second, err := MachineID()
	if err != nil {
		t.Fatalf("MachineID() second call error = %v", err)
	}

	if first != second {
		t.Fatalf("MachineID() mismatch: first=%q second=%q", first, second)
	}

	var record machineIdentityRecord
	path := filepath.Join(tempDir, "machine-id.json")
	if err := readJSON(path, &record); err != nil {
		t.Fatalf("readJSON(%q) error = %v", path, err)
	}

	if record.MachineID == "" {
		t.Fatal("machine-id.json stored an empty machine_id")
	}

	if first != record.MachineID {
		t.Fatalf("MachineID() = %q, want stored value %q", first, record.MachineID)
	}

	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidPattern.MatchString(first) {
		t.Fatalf("MachineID() = %q, want UUIDv4 format", first)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", path, err)
	}

	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("machine-id.json perms = %o, want 600", got)
	}
}

func TestShouldVerifyProcessLiveness(t *testing.T) {
	tests := []struct {
		name       string
		targetNS   string
		currentNS  string
		wantVerify bool
	}{
		{name: "legacy record", targetNS: "", currentNS: "", wantVerify: true},
		{name: "same namespace", targetNS: "pid:[4026533000]", currentNS: "pid:[4026533000]", wantVerify: true},
		{name: "different namespace", targetNS: "pid:[4026533000]", currentNS: "pid:[4026534000]", wantVerify: false},
		{name: "cannot compare current namespace", targetNS: "pid:[4026533000]", currentNS: "", wantVerify: false},
	}

	for _, tt := range tests {
		if got := shouldVerifyProcessLiveness(tt.targetNS, tt.currentNS); got != tt.wantVerify {
			t.Fatalf("%s: shouldVerifyProcessLiveness(%q, %q) = %t, want %t", tt.name, tt.targetNS, tt.currentNS, got, tt.wantVerify)
		}
	}
}

func TestLoadSummarySkipsPIDCheckAcrossNamespaces(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DIALTONE_WATCHER_HOME", tempDir)

	summary := Summary{
		PID:          999999,
		PIDNamespace: "pid:[4026533000]",
		Running:      true,
		PollCount:    3,
	}
	if err := writeSummary(summary); err != nil {
		t.Fatalf("writeSummary() error = %v", err)
	}

	loaded, err := LoadSummary()
	if err != nil {
		t.Fatalf("LoadSummary() error = %v", err)
	}
	if !loaded.Running {
		t.Fatalf("LoadSummary().Running = false, want true")
	}
	if loaded.PID != summary.PID {
		t.Fatalf("LoadSummary().PID = %d, want %d", loaded.PID, summary.PID)
	}
}
