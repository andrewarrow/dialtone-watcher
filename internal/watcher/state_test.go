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
