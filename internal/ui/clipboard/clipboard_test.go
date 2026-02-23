package clipboard

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestWriteNoPanic(t *testing.T) {
	// Redirect stderr to avoid polluting test output with OSC52 sequences
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		w.Close()
		r.Close()
		os.Stderr = origStderr
	}()

	// Should not panic — clipboard.WriteAll may fail in CI, but Write
	// tries OSC52 first which writes to stderr and should succeed.
	Write("test")
}

func TestWriteEmptyString(t *testing.T) {
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		w.Close()
		r.Close()
		os.Stderr = origStderr
	}()

	Write("")
}

func TestOSC52Encoding(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "hello"},
		{"with spaces", "hello world"},
		{"multiline", "line1\nline2\nline3"},
		{"unicode", "こんにちは"},
		{"empty", ""},
		{"special chars", "foo\tbar\nbaz\"qux"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origStderr := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("os.Pipe: %v", err)
			}
			os.Stderr = w

			err = writeOSC52(tt.input)
			w.Close()
			os.Stderr = origStderr

			if err != nil {
				r.Close()
				t.Fatalf("writeOSC52 returned error: %v", err)
			}

			buf := make([]byte, 4096)
			n, _ := r.Read(buf)
			r.Close()
			got := string(buf[:n])

			wantB64 := base64.StdEncoding.EncodeToString([]byte(tt.input))
			want := fmt.Sprintf("\x1b]52;c;%s\x07", wantB64)

			if got != want {
				t.Errorf("OSC52 mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}
}

func TestOSC52SequenceFormat(t *testing.T) {
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	err = writeOSC52("test data")
	w.Close()
	os.Stderr = origStderr

	if err != nil {
		r.Close()
		t.Fatalf("writeOSC52 returned error: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	got := string(buf[:n])

	if !strings.HasPrefix(got, "\x1b]52;c;") {
		t.Errorf("expected OSC52 prefix \\x1b]52;c; but got %q", got[:min(len(got), 10)])
	}
	if !strings.HasSuffix(got, "\x07") {
		t.Errorf("expected BEL suffix \\x07 but got %q", got[max(0, len(got)-5):])
	}
}
