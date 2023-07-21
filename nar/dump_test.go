package nar

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	slashpath "path"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
)

func TestDumper(t *testing.T) {
	for _, test := range narTests {
		if test.err || test.ignoreContents {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			fsys := make(fstest.MapFS)
			symlinks := make(map[string]string)
			for _, ent := range test.want {
				path := slashpath.Join("root", ent.header.Path)
				fsys[path] = &fstest.MapFile{
					Mode: ent.header.Mode,
					Data: []byte(ent.data),
				}
				if ent.header.Mode.Type() == fs.ModeSymlink {
					symlinks[path] = ent.header.LinkTarget
				}
			}
			d := &Dumper{
				ReadLink: func(path string) (string, error) {
					target, ok := symlinks[path]
					if !ok {
						return "", &fs.PathError{
							Op:   "readlink",
							Path: path,
							Err:  fs.ErrInvalid,
						}
					}
					return target, nil
				},
			}

			var buf bytes.Buffer
			if err := d.Dump(&buf, fsys, "root"); err != nil {
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

	t.Run("UnknownType", func(t *testing.T) {
		fsys := fstest.MapFS{
			"a": &fstest.MapFile{
				Mode: fs.ModeNamedPipe | 0o644,
			},
		}
		err := new(Dumper).Dump(io.Discard, fsys, "a")
		if err == nil {
			t.Fatal("DumpPath did not return an error")
		}
		const want = "unknown type"
		if got := err.Error(); !strings.Contains(got, want) {
			t.Errorf("DumpPath(...) = %s; did not contain %q", got, want)
		}
	})
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
