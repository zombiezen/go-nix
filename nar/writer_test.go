package nar

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
				if ent.header.Mode.IsRegular() {
					if got := nw.Offset(); got != ent.header.ContentOffset {
						t.Errorf("Offset() = %d; want %d", got, ent.header.ContentOffset)
					}
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

	t.Run("MissingParentDirectories", func(t *testing.T) {
		got := new(bytes.Buffer)
		nw := NewWriter(got)
		// write a directory node
		err := nw.WriteHeader(&Header{
			Mode: fs.ModeDir,
		})
		if err != nil {
			t.Fatal(err)
		}

		err = nw.WriteHeader(&Header{
			Path:       "foo/b",
			Mode:       fs.ModeSymlink,
			LinkTarget: "foo",
		})
		if err != nil {
			t.Fatal(err)
		}

		err = nw.WriteHeader(&Header{
			Path:       "foo-a",
			Mode:       fs.ModeSymlink,
			LinkTarget: "foo",
		})
		if err != nil {
			t.Fatal(err)
		}

		if err := nw.Close(); err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("testdata", "nested-dir-and-common-prefix.nar"))
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got.Bytes(), cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("-want +got:\n%s", diff)
		}
	})

	t.Run("MissingRootDirectoryForChildDirectory", func(t *testing.T) {
		got := new(bytes.Buffer)
		nw := NewWriter(got)

		err := nw.WriteHeader(&Header{
			Path: "foo",
			Mode: fs.ModeDir,
		})
		if err != nil {
			t.Fatal(err)
		}

		err = nw.WriteHeader(&Header{
			Path:       "foo/b",
			Mode:       fs.ModeSymlink,
			LinkTarget: "foo",
		})
		if err != nil {
			t.Fatal(err)
		}

		err = nw.WriteHeader(&Header{
			Path:       "foo-a",
			Mode:       fs.ModeSymlink,
			LinkTarget: "foo",
		})
		if err != nil {
			t.Fatal(err)
		}

		if err := nw.Close(); err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("testdata", "nested-dir-and-common-prefix.nar"))
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got.Bytes(), cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("-want +got:\n%s", diff)
		}
	})

	t.Run("TransitionViaSymlink", func(t *testing.T) {
		nw := NewWriter(io.Discard)
		// write a directory node
		err := nw.WriteHeader(&Header{
			Mode: fs.ModeDir | 0o555,
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink node for "/a"
		err = nw.WriteHeader(&Header{
			Path:       "a",
			Mode:       fs.ModeSymlink | 0o777,
			LinkTarget: "doesntmatter",
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink "/a/b", which should fail, as a was a symlink, not directory
		err = nw.WriteHeader(&Header{
			Path:       "a/b",
			Mode:       fs.ModeSymlink | 0o777,
			LinkTarget: "doesntmatter",
		})
		if err == nil {
			t.Error("WriteHeader did not return an error")
		}
	})

	t.Run("NotLexicographicallySorted", func(t *testing.T) {
		nw := NewWriter(io.Discard)
		// write a directory node
		err := nw.WriteHeader(&Header{
			Mode: fs.ModeDir | 0o555,
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/b"
		err = nw.WriteHeader(&Header{
			Path:       "b",
			Mode:       fs.ModeSymlink | 0o777,
			LinkTarget: "foo",
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/a"
		err = nw.WriteHeader(&Header{
			Path:       "a",
			Mode:       fs.ModeSymlink | 0o777,
			LinkTarget: "foo",
		})
		if err == nil {
			t.Error("WriteHeader did not return an error")
		}
	})

	t.Run("DuplicateNames", func(t *testing.T) {
		nw := NewWriter(io.Discard)
		// write a directory node
		err := nw.WriteHeader(&Header{
			Mode: fs.ModeDir | 0o555,
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/a"
		err = nw.WriteHeader(&Header{
			Path:       "a",
			Mode:       fs.ModeSymlink | 0o777,
			LinkTarget: "foo",
		})
		if err != nil {
			t.Fatal(err)
		}

		// write a symlink for "/a"
		err = nw.WriteHeader(&Header{
			Path:       "a",
			Mode:       fs.ModeSymlink | 0o777,
			LinkTarget: "foo",
		})
		if err == nil {
			t.Error("WriteHeader did not return an error")
		}
	})
}

func TestSparseWriter(t *testing.T) {
	// Test the sparse allocator mode produces similar results to regular writing.

	tempDir := t.TempDir()

	for _, test := range narTests {
		if test.ignoreContents || test.err {
			continue
		}
		t.Run(fmt.Sprintf("%sWithSparseAllocation", test.name), func(t *testing.T) {
			// Create a temp NAR file
			narPath := filepath.Join(tempDir, fmt.Sprintf("%s.nar", t.Name()))
			if err := os.MkdirAll(filepath.Dir(narPath), os.FileMode(0777)); err != nil {
				t.Fatal(err)
			}
			// Open as create/write only
			narFile, err := os.OpenFile(narPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(0777))
			if err != nil {
				t.Fatal(err)
			}
			defer narFile.Close()

			// Unlike the regular test, we should read 0's of the correct length.
			want, err := os.ReadFile(filepath.Join("testdata", test.dataFile))
			if err != nil {
				t.Fatal(err)
			}

			cb := func(hdr Header) error {
				// For the test case, read the file content out of the example NAR and
				// emplace it into the pre-allocated space here.
				if _, err := narFile.Seek(hdr.ContentOffset, io.SeekStart); err != nil {
					return err
				}
				if _, err := io.Copy(narFile, bytes.NewBuffer(want[hdr.ContentOffset:hdr.ContentOffset+hdr.Size])); err != nil {
					return err
				}
				// No need to reset the pointer since it should already be properly positioned in this case.
				return nil
			}

			nw := NewWriter(narFile, SparseAllocate(cb))
			for i, ent := range test.want {
				if err := nw.WriteHeader(ent.header); err != nil {
					t.Errorf("WriteHeader#%d(%+v): %v", i+1, ent.header, err)
				}
				if ent.header.Mode.IsRegular() {
					if got := nw.Offset(); got != ent.header.ContentOffset {
						t.Errorf("Offset() = %d; want %d", got, ent.header.ContentOffset)
					}
				}
			}
			if err := nw.Close(); err != nil {
				t.Error("Close:", err)
			}

			got, err := os.ReadFile(narPath)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
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

const bufWriterSize = len(BufWriter{}.buf)

func TestBufWriterString(t *testing.T) {
	const overflowSize = bufWriterSize + 1

	t.Run("LongString", func(t *testing.T) {
		buf := new(bytes.Buffer)
		bw := &BufWriter{w: buf}
		s := strings.Repeat("x", overflowSize)
		bw.String(s)
		bw.Flush()

		want := make([]byte, 8+overflowSize+(StringAlign-1))
		binary.LittleEndian.PutUint64(want, uint64(overflowSize))
		copy(want[8:], s)
		if diff := cmp.Diff(want, buf.Bytes()); diff != "" {
			t.Errorf("-want +got:\n%s", diff)
		}
	})

	t.Run("LongStringWithoutWriteString", func(t *testing.T) {
		buf := new(bytes.Buffer)
		bw := &BufWriter{w: onlyWriter{buf}}
		s := strings.Repeat("x", overflowSize)
		bw.String(s)
		bw.Flush()

		want := make([]byte, 8+overflowSize+(StringAlign-1))
		binary.LittleEndian.PutUint64(want, uint64(overflowSize))
		copy(want[8:], s)
		if diff := cmp.Diff(want, buf.Bytes()); diff != "" {
			t.Errorf("-want +got:\n%s", diff)
		}
	})

	t.Run("StringAtEndOfBuffer", func(t *testing.T) {
		const badBits = 0xbaadf00dbaadf00d

		buf := new(bytes.Buffer)
		bw := &BufWriter{w: onlyWriter{buf}}
		// Fill buffer with garbage.
		for i := 0; i < bufWriterSize/8; i++ {
			bw.Uint64(badBits)
		}
		// Write bytes to misalign by 1 byte.
		if _, err := bw.Write(bytes.Repeat([]byte("x"), 7)); err != nil {
			t.Fatal(err)
		}
		bw.Pad()
		// Fill to right next to end.
		const yLen = bufWriterSize - 8*3
		bw.String(strings.Repeat("y", yLen))
		bw.String("z")
		bw.Flush()

		want := make([]byte, 0, bufWriterSize+8+bufWriterSize)
		for i := 0; i < bufWriterSize/8; i++ {
			want = binary.LittleEndian.AppendUint64(want, badBits)
		}
		for i := 0; i < 7; i++ {
			want = append(want, 'x')
		}
		want = append(want, 0)
		want = binary.LittleEndian.AppendUint64(want, uint64(yLen))
		for i := 0; i < yLen; i++ {
			want = append(want, 'y')
		}
		want = binary.LittleEndian.AppendUint64(want, 1)
		want = append(want, 'z', 0, 0, 0, 0, 0, 0, 0)

		if diff := cmp.Diff(want, buf.Bytes()); diff != "" {
			t.Errorf("-want +got:\n%s", diff)
		}
	})
}

func TestBufWriterUint64(t *testing.T) {
	buf := new(bytes.Buffer)
	bw := &BufWriter{
		w:      buf,
		bufLen: int16(bufWriterSize) - 1,
	}
	bw.Uint64(42)
	bw.Flush()

	want := bytes.Repeat([]byte{0}, bufWriterSize)
	want = binary.LittleEndian.AppendUint64(want, 42)
	if diff := cmp.Diff(want, buf.Bytes()); diff != "" {
		t.Errorf("-want +got:\n%s", diff)
	}
}

func TestBufWriterPad(t *testing.T) {
	buf := new(bytes.Buffer)
	const initialOffset = 2
	bw := &BufWriter{
		w:      buf,
		off:    initialOffset,
		bufLen: int16(bufWriterSize) - 1,
	}
	bw.Pad()
	bw.Flush()

	want := bytes.Repeat([]byte{0}, bufWriterSize+StringAlign-initialOffset)
	if diff := cmp.Diff(want, buf.Bytes()); diff != "" {
		t.Errorf("-want +got:\n%s", diff)
	}
}

type onlyWriter struct {
	w io.Writer
}

func (w onlyWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
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
		{
			oldPath:  "share/locale/be/LC_MESSAGES",
			oldIsDir: true,
			newPath:  "share/locale/bg",
			pop:      2,
			newDirs:  "",
		},
	}
	for _, test := range tests {
		pop, newDirs, err := TreeDelta(test.oldPath, test.oldIsDir, test.newPath)
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
