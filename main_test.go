package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunNoArgsPrintsMachineIDAndHelp(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DIALTONE_WATCHER_HOME", tempDir)

	output := captureStdout(t, func() {
		if err := run(nil); err != nil {
			t.Fatalf("run(nil) error = %v", err)
		}
	})

	if !strings.Contains(output, "Machine ID: ") {
		t.Fatalf("run(nil) output missing machine id:\n%s", output)
	}
	if !strings.Contains(output, "Machine ID File: "+filepath.Join(tempDir, "machine-id.json")) {
		t.Fatalf("run(nil) output missing machine id file path:\n%s", output)
	}
	if !strings.Contains(output, "Usage:") {
		t.Fatalf("run(nil) output missing help text:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}

	os.Stdout = writer
	defer func() {
		os.Stdout = original
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	var buffer bytes.Buffer
	if _, err := io.Copy(&buffer, reader); err != nil {
		t.Fatalf("io.Copy() error = %v", err)
	}

	return buffer.String()
}
