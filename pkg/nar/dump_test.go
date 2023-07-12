package nar_test

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/nix-community/go-nix/pkg/nar"
)

func TestDumpPath(t *testing.T) {
	for _, test := range narTests {
		if test.err || test.ignoreContents {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			for _, ent := range test.want {
				if ent.header.Mode&0o111 != 0 && runtime.GOOS == "windows" {
					t.Skipf("Cannot test on Windows due to %q being executable", ent.header.Path)
				}
			}

			// Set up in filesystem.
			dir := t.TempDir()
			for _, ent := range test.want {
				path := filepath.Join(dir, "root", filepath.FromSlash(ent.header.Path))
				switch ent.header.Mode.Type() {
				case 0:
					if err := os.WriteFile(path, []byte(ent.data), ent.header.Mode.Perm()); err != nil {
						t.Fatal(err)
					}
				case fs.ModeDir:
					if err := os.Mkdir(path, 0o777); err != nil {
						t.Fatal(err)
					}
				case fs.ModeSymlink:
					if err := os.Symlink(ent.header.LinkTarget, path); err != nil {
						t.Fatal(err)
					}
				default:
					t.Fatalf("For path %q, unknown mode %q", ent.header.Path, ent.header.Mode)
				}
			}

			var buf bytes.Buffer
			if err := DumpPath(&buf, filepath.Join(dir, "root")); err != nil {
				t.Error(err)
			}

			want, err := os.ReadFile(filepath.Join("testdata", test.dataFile))
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, buf.Bytes()); diff != "" {
				t.Errorf("-want +got:\n%s", diff)
			}
		})
	}
}

func TestDumpPathFilter(t *testing.T) {
	t.Run("unfiltered", func(t *testing.T) {
		tmpDir := t.TempDir()
		p := filepath.Join(tmpDir, "a")

		err := os.WriteFile(p, []byte{0x1}, 0o444)
		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer

		err = DumpPathFilter(&buf, p, func(name string, mode fs.FileMode) bool {
			if name != p {
				t.Errorf("name = %q; want %q", name, p)
			}
			if mode.Type() != 0 {
				t.Errorf("nodeType = %v; want %v", mode, mode&^fs.ModeType)
			}

			return true
		})
		if err != nil {
			t.Error("DumpPathFilter:", err)
		}

		want, err := os.ReadFile(filepath.Join("testdata", "1byte-regular.nar"))
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, buf.Bytes()); diff != "" {
			t.Errorf("-want +got:\n%s", diff)
		}
	})

	t.Run("filtered", func(t *testing.T) {
		tmpDir := t.TempDir()
		p := filepath.Join(tmpDir, "a")

		err := os.WriteFile(p, []byte{0x1}, 0o444)
		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer

		err = DumpPathFilter(&buf, tmpDir, func(name string, mode fs.FileMode) bool {
			return name != p
		})
		if err != nil {
			t.Error("DumpPathFilter:", err)
		}

		want, err := os.ReadFile(filepath.Join("testdata", "empty-directory.nar"))
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, buf.Bytes()); diff != "" {
			t.Errorf("-want +got:\n%s", diff)
		}
	})
}

func BenchmarkDumpPath(b *testing.B) {
	b.Run("testdata", func(b *testing.B) {
		bc := new(byteCounter)
		err := DumpPath(bc, "testdata")
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		b.SetBytes(bc.n)

		for i := 0; i < b.N; i++ {
			err := DumpPath(io.Discard, "testdata")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

type byteCounter struct {
	n int64
}

func (bc *byteCounter) Write(p []byte) (n int, err error) {
	bc.n += int64(len(p))
	return len(p), nil
}
