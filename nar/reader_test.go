package nar

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestReader(t *testing.T) {
	t.Run("OnlyMagic", func(t *testing.T) {
		r := NewReader(strings.NewReader(
			"\x0d\x00\x00\x00\x00\x00\x00\x00" +
				"nix-archive-1\x00\x00\x00",
		))
		hdr, err := r.Next()
		if err == nil {
			t.Fatalf("r.Next() = %+v, <nil>; want _, <error>", hdr)
		}
		if errors.Is(err, io.EOF) {
			t.Logf("r.Next() = _, %v; want _, <!EOF>", err)
		} else {
			t.Logf("r.Next() = _, %v", err)
		}
	})

	t.Run("EmptyFile", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "empty-file.nar"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := NewReader(f)

		got, err := r.Next()
		if err != nil {
			t.Fatal("r.Next #1:", err)
		}
		want := &Header{
			Mode: 0o444,
			Size: 0,
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("header (-want +got):\n%s", diff)
		}
		// TODO(now): Read all.

		got, err = r.Next()
		if err != io.EOF {
			t.Errorf("r.Next() #2 = %+v, %v; want _, %v", got, err, io.EOF)
		}
	})
}
