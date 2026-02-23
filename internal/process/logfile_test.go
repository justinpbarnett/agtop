package process

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFollowReader_ReadsExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Write some data
	if err := os.WriteFile(path, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fr := NewFollowReader(ctx, f)
	defer fr.Close()

	buf := make([]byte, 64)
	n, err := fr.Read(buf)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(buf[:n]) != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got %q", string(buf[:n]))
	}
}

func TestFollowReader_WaitsForNewData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Create empty file
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Open for reading
	rf, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fr := NewFollowReader(ctx, rf)
	defer fr.Close()

	// Write data after a short delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		wf, _ := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
		wf.Write([]byte("delayed data\n"))
		wf.Close()
	}()

	buf := make([]byte, 64)
	n, err := fr.Read(buf)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(buf[:n]) != "delayed data\n" {
		t.Errorf("expected 'delayed data\\n', got %q", string(buf[:n]))
	}
}

func TestFollowReader_StopsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Create empty file
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	rf, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	fr := NewFollowReader(ctx, rf)

	// Cancel after a short delay
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	buf := make([]byte, 64)
	start := time.Now()
	_, err = fr.Read(buf)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected EOF error after cancel")
	}
	// Should have returned within ~200ms, not hanging forever
	if elapsed > 2*time.Second {
		t.Errorf("Read took too long after cancel: %v", elapsed)
	}
}

func TestCreateLogFiles(t *testing.T) {
	dir := t.TempDir()

	lf, err := CreateLogFiles(dir, "001")
	if err != nil {
		t.Fatal(err)
	}
	defer lf.Close()

	expectedStdout := filepath.Join(dir, "001.stdout")
	expectedStderr := filepath.Join(dir, "001.stderr")

	if lf.StdoutPath() != expectedStdout {
		t.Errorf("stdout path: got %s, want %s", lf.StdoutPath(), expectedStdout)
	}
	if lf.StderrPath() != expectedStderr {
		t.Errorf("stderr path: got %s, want %s", lf.StderrPath(), expectedStderr)
	}

	// Files should exist
	if _, err := os.Stat(expectedStdout); err != nil {
		t.Errorf("stdout file not created: %v", err)
	}
	if _, err := os.Stat(expectedStderr); err != nil {
		t.Errorf("stderr file not created: %v", err)
	}

	// Should be writable
	if _, err := lf.StdoutWriter().Write([]byte("test\n")); err != nil {
		t.Errorf("stdout write failed: %v", err)
	}
}

func TestOpenLogFiles(t *testing.T) {
	dir := t.TempDir()

	stdoutPath := filepath.Join(dir, "001.stdout")
	stderrPath := filepath.Join(dir, "001.stderr")

	// Create files with content
	os.WriteFile(stdoutPath, []byte("stdout content\n"), 0o644)
	os.WriteFile(stderrPath, []byte("stderr content\n"), 0o644)

	lf, err := OpenLogFiles(stdoutPath, stderrPath)
	if err != nil {
		t.Fatal(err)
	}
	defer lf.Close()

	// Should be readable
	buf := make([]byte, 64)
	n, err := lf.stdout.Read(buf)
	if err != nil {
		t.Errorf("stdout read failed: %v", err)
	}
	if string(buf[:n]) != "stdout content\n" {
		t.Errorf("expected 'stdout content\\n', got %q", string(buf[:n]))
	}
}

func TestRemoveLogFiles(t *testing.T) {
	dir := t.TempDir()

	stdoutPath := filepath.Join(dir, "001.stdout")
	stderrPath := filepath.Join(dir, "001.stderr")

	os.WriteFile(stdoutPath, []byte("data"), 0o644)
	os.WriteFile(stderrPath, []byte("data"), 0o644)

	RemoveLogFiles(stdoutPath, stderrPath)

	if _, err := os.Stat(stdoutPath); !os.IsNotExist(err) {
		t.Error("stdout file should be deleted")
	}
	if _, err := os.Stat(stderrPath); !os.IsNotExist(err) {
		t.Error("stderr file should be deleted")
	}
}
