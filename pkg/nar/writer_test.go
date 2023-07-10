package nar_test

import (
	"bytes"
	"io"
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
			nw, err := NewWriter(buf)
			if err != nil {
				t.Fatal(err)
			}
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
		nw, err := NewWriter(io.Discard)
		if err != nil {
			t.Fatal(err)
		}
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
		w, err := NewWriter(buf)
		if err != nil {
			b.Fatal(err)
		}

		err = w.WriteHeader(&Header{
			Path: "/",
			Type: TypeDirectory,
		})
		if err != nil {
			b.Fatal(err)
		}

		err = w.WriteHeader(&Header{
			Path: "/a.txt",
			Type: TypeRegular,
			Size: 4,
		})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.WriteString(w, "AAA\n"); err != nil {
			b.Fatal(err)
		}

		err = w.WriteHeader(&Header{
			Path: "/bin",
			Type: TypeDirectory,
		})
		if err != nil {
			b.Fatal(err)
		}

		err = w.WriteHeader(&Header{
			Path: "/bin/hello.sh",
			Type: TypeRegular,
			Size: int64(len(miniDRVScriptData)),
		})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.WriteString(w, miniDRVScriptData); err != nil {
			b.Fatal(err)
		}

		err = w.WriteHeader(&Header{
			Path: "/hello.txt",
			Type: TypeRegular,
			Size: int64(len(helloWorld)),
		})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.WriteString(w, helloWorld); err != nil {
			b.Fatal(err)
		}

		if err := w.Close(); err != nil {
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
	t.Run("missing directory in between", func(t *testing.T) {
		var buf bytes.Buffer
		nw, err := NewWriter(&buf)
		if err != nil {
			t.Fatal(err)
		}

		// write a directory node
		err = nw.WriteHeader(&Header{
			Path: "/",
			Type: TypeDirectory,
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink "a/foo", but missing the directory node "a" in between should error
		err = nw.WriteHeader(&Header{
			Path:       "/a/foo",
			Type:       TypeSymlink,
			LinkTarget: "doesntmatter",
		})
		if err == nil {
			t.Error("WriteHeader did not return an error")
		}
	})

	t.Run("missing directory at the beginning, writing another directory", func(t *testing.T) {
		var buf bytes.Buffer
		nw, err := NewWriter(&buf)
		if err != nil {
			t.Fatal(err)
		}

		// write a directory node for "/a" without writing the one for "/"
		err = nw.WriteHeader(&Header{
			Path: "/a",
			Type: TypeDirectory,
		})
		if err == nil {
			t.Error("WriteHeader did not return an error")
		}
	})

	t.Run("missing directory at the beginning, writing a symlink", func(t *testing.T) {
		var buf bytes.Buffer
		nw, err := NewWriter(&buf)
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "a" without writing the directory one for ""
		err = nw.WriteHeader(&Header{
			Path:       "/a",
			Type:       TypeSymlink,
			LinkTarget: "foo",
		})
		if err == nil {
			t.Error("WriteHeader did not return an error")
		}
	})

	t.Run("transition via a symlink, not directory", func(t *testing.T) {
		var buf bytes.Buffer
		nw, err := NewWriter(&buf)
		if err != nil {
			t.Fatal(err)
		}

		// write a directory node
		err = nw.WriteHeader(&Header{
			Path: "/",
			Type: TypeDirectory,
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink node for "/a"
		err = nw.WriteHeader(&Header{
			Path:       "/a",
			Type:       TypeSymlink,
			LinkTarget: "doesntmatter",
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink "/a/b", which should fail, as a was a symlink, not directory
		err = nw.WriteHeader(&Header{
			Path:       "/a/b",
			Type:       TypeSymlink,
			LinkTarget: "doesntmatter",
		})
		if err == nil {
			t.Error("WriteHeader did not return an error")
		}
	})

	t.Run("not lexicographically sorted", func(t *testing.T) {
		var buf bytes.Buffer
		nw, err := NewWriter(&buf)
		if err != nil {
			t.Fatal(err)
		}

		// write a directory node
		err = nw.WriteHeader(&Header{
			Path: "/",
			Type: TypeDirectory,
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/b"
		err = nw.WriteHeader(&Header{
			Path:       "/b",
			Type:       TypeSymlink,
			LinkTarget: "foo",
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/a"
		err = nw.WriteHeader(&Header{
			Path:       "/a",
			Type:       TypeSymlink,
			LinkTarget: "foo",
		})
		if err == nil {
			t.Error("WriteHeader did not return an error")
		}
	})

	t.Run("not lexicographically sorted, but the same", func(t *testing.T) {
		var buf bytes.Buffer
		nw, err := NewWriter(&buf)
		if err != nil {
			t.Fatal(err)
		}

		// write a directory node
		err = nw.WriteHeader(&Header{
			Path: "/",
			Type: TypeDirectory,
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/a"
		err = nw.WriteHeader(&Header{
			Path:       "/a",
			Type:       TypeSymlink,
			LinkTarget: "foo",
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/a"
		err = nw.WriteHeader(&Header{
			Path:       "/a",
			Type:       TypeSymlink,
			LinkTarget: "foo",
		})
		if err == nil {
			t.Error("WriteHeader did not return an error")
		}
	})

	t.Run("lexicographically sorted with nested directory and common prefix", func(t *testing.T) {
		var buf bytes.Buffer
		nw, err := NewWriter(&buf)
		if err != nil {
			t.Fatal(err)
		}

		// write a directory node
		err = nw.WriteHeader(&Header{
			Path: "/",
			Type: TypeDirectory,
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a directory node with name "/foo"
		err = nw.WriteHeader(&Header{
			Path: "/foo",
			Type: TypeDirectory,
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/foo/b"
		err = nw.WriteHeader(&Header{
			Path:       "/foo/b",
			Type:       TypeSymlink,
			LinkTarget: "foo",
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/foo-a"
		err = nw.WriteHeader(&Header{
			Path:       "/foo-a",
			Type:       TypeSymlink,
			LinkTarget: "foo",
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}
