package nar

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestWriter(t *testing.T) {
	type writerTest struct {
		name   string
		golden string
		do     func(*testing.T, *Writer)
	}

	var tests []writerTest
	for i := range narTests {
		test := narTests[i] // capture variable
		tests = append(tests, writerTest{
			name:   test.name,
			golden: test.dataFile,
			do: func(t *testing.T, w *Writer) {
				for i, ent := range test.want {
					if err := w.WriteHeader(ent.header); err != nil {
						t.Errorf("WriteHeader#%d(%+v): %v", i+1, ent.header, err)
					}
					if ent.data != "" {
						if _, err := io.WriteString(w, ent.data); err != nil {
							t.Errorf("io.WriteString#%d(w, %q): %v", i+1, ent.data, err)
						}
					}
				}
				if err := w.Close(); err != nil {
					t.Error("Close:", err)
				}
			},
		})
	}
	tests = append(tests,
		writerTest{
			name:   "TreeFilesOnly",
			golden: "mini-drv.nar",
			do: func(t *testing.T, w *Writer) {
				err := w.WriteHeader(&Header{Path: "a.txt", Size: 4})
				if err != nil {
					t.Error(err)
				}
				if _, err := io.WriteString(w, "AAA\n"); err != nil {
					t.Error(err)
				}

				err = w.WriteHeader(&Header{
					Path: "bin/hello.sh",
					Mode: 0o700,
					Size: int64(len(helloScriptData)),
				})
				if err != nil {
					t.Error(err)
				}
				if _, err := io.WriteString(w, helloScriptData); err != nil {
					t.Error(err)
				}

				err = w.WriteHeader(&Header{
					Path: "hello.txt",
					Size: 14,
				})
				if err != nil {
					t.Error(err)
				}
				if _, err := io.WriteString(w, "Hello, World!\n"); err != nil {
					t.Error(err)
				}

				if err := w.Close(); err != nil {
					t.Error(err)
				}
			},
		},
		writerTest{
			name: "ImmediateClose",
			do: func(t *testing.T, w *Writer) {
				if err := w.Close(); err == nil {
					t.Error("Close did not return an error")
				} else {
					t.Log("Close:", err)
				}
			},
		},
	)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			test.do(t, NewWriter(buf))
			var want []byte
			if test.golden != "" {
				var err error
				want, err = os.ReadFile(filepath.Join("testdata", test.golden))
				if err != nil {
					t.Fatal(err)
				}
			}
			if diff := cmp.Diff(want, buf.Bytes(), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("-want +got:\n%s", diff)
			}
		})
	}
}

func BenchmarkWriter(b *testing.B) {
	buf := new(bytes.Buffer)

	// First iteration is a warmup. See below.
	for i := 0; i < b.N+1; i++ {
		buf.Reset()
		w := NewWriter(buf)

		err := w.WriteHeader(&Header{Path: "a.txt", Size: 4})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.WriteString(w, "AAA\n"); err != nil {
			b.Fatal(err)
		}

		err = w.WriteHeader(&Header{
			Path: "bin/hello.sh",
			Mode: 0o700,
			Size: int64(len(helloScriptData)),
		})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.WriteString(w, helloScriptData); err != nil {
			b.Fatal(err)
		}

		err = w.WriteHeader(&Header{
			Path: "hello.txt",
			Size: 14,
		})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.WriteString(w, "Hello, World!\n"); err != nil {
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

func TestTreeDelta(t *testing.T) {
	tests := []struct {
		oldPath  string
		oldIsDir bool
		newPath  string
		pop      int
		newDirs  string
		err      bool
	}{
		{
			oldPath:  "",
			oldIsDir: true,
			newPath:  "foo.txt",
			pop:      0,
			newDirs:  "",
		},
		{
			oldPath:  "",
			oldIsDir: false,
			newPath:  "",
			pop:      0,
			newDirs:  "",
		},
		{
			oldPath:  "bar.txt",
			oldIsDir: false,
			newPath:  "foo.txt",
			pop:      0,
			newDirs:  "",
		},
		{
			oldPath:  "foo.txt",
			oldIsDir: false,
			newPath:  "bar.txt",
			err:      true,
		},
		{
			oldPath:  "",
			oldIsDir: true,
			newPath:  "a/foo.txt",
			pop:      0,
			newDirs:  "a",
		},
		{
			oldPath:  "",
			oldIsDir: true,
			newPath:  "a/b/foo.txt",
			pop:      0,
			newDirs:  "a/b",
		},
		{
			oldPath:  "0/x",
			oldIsDir: false,
			newPath:  "a/b/foo.txt",
			pop:      1,
			newDirs:  "a/b",
		},
		{
			oldPath:  "x/y",
			oldIsDir: false,
			newPath:  "a/foo.txt",
			err:      true,
		},
		{
			oldPath:  "x",
			oldIsDir: true,
			newPath:  "x/foo.txt",
			pop:      0,
			newDirs:  "",
		},
		{
			oldPath:  "x",
			oldIsDir: false,
			newPath:  "x/foo.txt",
			err:      true,
		},
	}
	for _, test := range tests {
		pop, newDirs, err := treeDelta(test.oldPath, test.oldIsDir, test.newPath)
		if pop != test.pop || newDirs != test.newDirs || (err != nil) != test.err {
			errString := "<nil>"
			if test.err {
				errString = "<error>"
			}
			t.Errorf("treeDelta(%q, %t, %q) = %d, %q, %v; want %d, %q, %s",
				test.oldPath, test.oldIsDir, test.newPath, pop, newDirs, err, test.pop, test.newDirs, errString)
		}
	}
}
