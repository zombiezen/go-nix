//go:build !windows
// +build !windows

package nar_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/nix-community/go-nix/pkg/nar"
)

// TestDumpPathUnknown makes sure calling DumpPath on a path with a fifo
// doesn't panic, but returns an error.
func TestDumpPathUnknown(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "a")

	err := syscall.Mkfifo(p, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	err = nar.DumpPath(&buf, p)
	if err == nil {
		t.Fatal("DumpPath did not return an error")
	}
	const want = "unknown type"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("DumpPath(...) = %s; did not contain %q", got, want)
	}
}
