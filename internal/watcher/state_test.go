package watcher

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestMachineIDPersistsAndUsesHashedValue(t *testing.T) {
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

	sum := sha256.Sum256([]byte(record.MachineID + "|dialtone-watcher"))
	want := hex.EncodeToString(sum[:])
	if first != want {
		t.Fatalf("MachineID() = %q, want %q", first, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", path, err)
	}

	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("machine-id.json perms = %o, want 600", got)
	}
}
