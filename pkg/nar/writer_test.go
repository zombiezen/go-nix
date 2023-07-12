package nar_test

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	. "github.com/nix-community/go-nix/pkg/nar"
)

func TestWriter(t *testing.T) {
	for _, test := range narTests {
		if test.ignoreContents || test.err {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			nw := NewWriter(buf)
			for i, ent := range test.want {
				if err := nw.WriteHeader(ent.header); err != nil {
					t.Errorf("WriteHeader#%d(%+v): %v", i+1, ent.header, err)
				}
				if ent.data != "" {
					if _, err := io.WriteString(nw, ent.data); err != nil {
						t.Errorf("io.WriteString#%d(w, %q): %v", i+1, ent.data, err)
					}
				}
			}
			if err := nw.Close(); err != nil {
				t.Error("Close:", err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", test.dataFile))
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, buf.Bytes(), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("-want +got:\n%s", diff)
			}
		})
	}

	t.Run("ImmediateClose", func(t *testing.T) {
		nw := NewWriter(io.Discard)
		if err := nw.Close(); err == nil {
			t.Error("Close did not return an error")
		} else {
			t.Log("Close:", err)
		}
	})
}

func BenchmarkWriter(b *testing.B) {
	buf := new(bytes.Buffer)

	// First iteration is a warmup. See below.
	for i := 0; i < b.N+1; i++ {
		buf.Reset()
		nw := NewWriter(buf)

		err := nw.WriteHeader(&Header{
			Path: "a.txt",
			Mode: 0o444,
			Size: 4,
		})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.WriteString(nw, "AAA\n"); err != nil {
			b.Fatal(err)
		}

		err = nw.WriteHeader(&Header{
			Path: "bin/hello.sh",
			Mode: 0o555,
			Size: int64(len(miniDRVScriptData)),
		})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.WriteString(nw, miniDRVScriptData); err != nil {
			b.Fatal(err)
		}

		err = nw.WriteHeader(&Header{
			Path: "hello.txt",
			Mode: 0o444,
			Size: int64(len(helloWorld)),
		})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.WriteString(nw, helloWorld); err != nil {
			b.Fatal(err)
		}

		if err := nw.Close(); err != nil {
			b.Fatal(err)
		}

		if i == 0 {
			// First iteration we're finding the buffer size.
			b.SetBytes(int64(buf.Len()))
			b.ResetTimer()
		}
	}
}

func TestWriterErrorsTransitions(t *testing.T) {
	t.Run("transition via a symlink, not directory", func(t *testing.T) {
		nw := NewWriter(io.Discard)

		// write a directory node
		err := nw.WriteHeader(&Header{
			Mode: fs.ModeDir,
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink node for "/a"
		err = nw.WriteHeader(&Header{
			Path:       "a",
			Mode:       fs.ModeSymlink,
			LinkTarget: "doesntmatter",
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink "/a/b", which should fail, as a was a symlink, not directory
		err = nw.WriteHeader(&Header{
			Path:       "a/b",
			Mode:       fs.ModeSymlink,
			LinkTarget: "doesntmatter",
		})
		if err == nil {
			t.Error("WriteHeader did not return an error")
		}
	})

	t.Run("not lexicographically sorted", func(t *testing.T) {
		nw := NewWriter(io.Discard)

		// write a directory node
		err := nw.WriteHeader(&Header{
			Mode: fs.ModeDir,
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/b"
		err = nw.WriteHeader(&Header{
			Path:       "b",
			Mode:       fs.ModeSymlink,
			LinkTarget: "foo",
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/a"
		err = nw.WriteHeader(&Header{
			Path:       "a",
			Mode:       fs.ModeSymlink,
			LinkTarget: "foo",
		})
		if err == nil {
			t.Error("WriteHeader did not return an error")
		}
	})

	t.Run("not lexicographically sorted, but the same", func(t *testing.T) {
		nw := NewWriter(io.Discard)

		// write a directory node
		err := nw.WriteHeader(&Header{
			Path: "",
			Mode: fs.ModeDir,
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/a"
		err = nw.WriteHeader(&Header{
			Path:       "a",
			Mode:       fs.ModeSymlink,
			LinkTarget: "foo",
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/a"
		err = nw.WriteHeader(&Header{
			Path:       "a",
			Mode:       fs.ModeSymlink,
			LinkTarget: "foo",
		})
		if err == nil {
			t.Error("WriteHeader did not return an error")
		}
	})
}
