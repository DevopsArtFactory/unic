package log

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesLogDirAndFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(configDirEnv, tmpDir)

	if err := Init(false); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	logPath := filepath.Join(tmpDir, appName, logDirName, logFileName)
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("log file not created: %v", err)
	}
}

func TestInfoWritesJSONToFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(configDirEnv, tmpDir)

	if err := Init(false); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Info("test", "hello world", "key", "value")
	Close()

	logPath := filepath.Join(tmpDir, appName, logDirName, logFileName)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("log entry is not valid JSON: %v\ndata: %s", err, data)
	}

	if entry["msg"] != "hello world" {
		t.Errorf("expected msg 'hello world', got %v", entry["msg"])
	}
	if entry["component"] != "test" {
		t.Errorf("expected component 'test', got %v", entry["component"])
	}
	if entry["key"] != "value" {
		t.Errorf("expected key 'value', got %v", entry["key"])
	}
}

func TestDebugNotWrittenWithoutVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(configDirEnv, tmpDir)

	if err := Init(false); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Debug("test", "should not appear")
	Close()

	logPath := filepath.Join(tmpDir, appName, logDirName, logFileName)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if len(data) > 0 {
		t.Errorf("expected empty log file for debug without verbose, got: %s", data)
	}
}

func TestDebugWrittenWithVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(configDirEnv, tmpDir)

	if err := Init(true); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Debug("test", "debug message")
	Close()

	logPath := filepath.Join(tmpDir, appName, logDirName, logFileName)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("log entry is not valid JSON: %v", err)
	}

	if entry["msg"] != "debug message" {
		t.Errorf("expected msg 'debug message', got %v", entry["msg"])
	}
}

func TestRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, logFileName)

	// Create a file that exceeds maxLogSize
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, maxLogSize+1)
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := rotate(logPath); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}

	// Original should be gone (renamed to .1)
	if _, err := os.Stat(logPath); err == nil {
		t.Error("original file should have been rotated")
	}

	rotated := logPath + ".1"
	if _, err := os.Stat(rotated); err != nil {
		t.Errorf("rotated file .1 should exist: %v", err)
	}
}

func TestRotationChain(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, logFileName)

	// Create .1 and .2 files, then a large current file
	for _, suffix := range []string{".1", ".2"} {
		if err := os.WriteFile(logPath+suffix, []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	data := make([]byte, maxLogSize+1)
	if err := os.WriteFile(logPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := rotate(logPath); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}

	// .1 should be new (from current), .2 should be old .1, .3 should be old .2
	for _, suffix := range []string{".1", ".2", ".3"} {
		if _, err := os.Stat(logPath + suffix); err != nil {
			t.Errorf("expected %s to exist: %v", suffix, err)
		}
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(configDirEnv, tmpDir)

	if err := Init(false); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	Close()
	Close() // should not panic
}
