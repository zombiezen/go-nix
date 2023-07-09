package nar

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type testEntry struct {
	header *Header
	data   string
}

const helloScriptData = "#!/bin/sh\n" +
	`cat "$(dirname "$0")/../hello.txt"` + "\n"

var narTests = []struct {
	name     string
	dataFile string
	want     []testEntry
}{
	{
		name:     "EmptyFile",
		dataFile: "empty-file.nar",
		want: []testEntry{
			{
				header: &Header{
					Mode: 0o444,
					Size: 0,
				},
			},
		},
	},
	{
		name:     "EmptyDirectory",
		dataFile: "empty-directory.nar",
		want: []testEntry{
			{
				header: &Header{
					Mode: 0o555 | fs.ModeDir,
				},
			},
		},
	},
	{
		name:     "TextFile",
		dataFile: "hello-world.nar",
		want: []testEntry{
			{
				header: &Header{
					Mode: 0o444,
					Size: 14,
				},
				data: "Hello, World!\n",
			},
		},
	},
	{
		name:     "Script",
		dataFile: "hello-script.nar",
		want: []testEntry{
			{
				header: &Header{
					Mode: 0o555,
					Size: 31,
				},
				data: "#!/bin/sh\necho 'Hello, World!'\n",
			},
		},
	},
	{
		name:     "Symlink",
		dataFile: "symlink.nar",
		want: []testEntry{
			{
				header: &Header{
					Mode:     fs.ModeSymlink | 0o777,
					Linkname: "foo/bar/baz",
				},
			},
		},
	},
	{
		name:     "Tree",
		dataFile: "mini-drv.nar",
		want: []testEntry{
			{
				header: &Header{
					Mode: fs.ModeDir | 0o555,
				},
			},
			{
				header: &Header{
					Path: "a.txt",
					Mode: 0o444,
					Size: 4,
				},
				data: "AAA\n",
			},
			{
				header: &Header{
					Path: "bin",
					Mode: fs.ModeDir | 0o555,
				},
			},
			{
				header: &Header{
					Path: "bin/hello.sh",
					Mode: 0o555,
					Size: int64(len(helloScriptData)),
				},
				data: helloScriptData,
			},
			{
				header: &Header{
					Path: "hello.txt",
					Mode: 0o444,
					Size: 14,
				},
				data: "Hello, World!\n",
			},
		},
	},
}

func TestReader(t *testing.T) {
	for _, test := range narTests {
		t.Run(test.name, func(t *testing.T) {
			f, err := os.Open(filepath.Join("testdata", test.dataFile))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			r := NewReader(f)

			for i := range test.want {
				gotHeader, err := r.Next()
				if err != nil {
					t.Fatalf("r.Next() #%d: %v", i+1, err)
				}
				if diff := cmp.Diff(test.want[i].header, gotHeader); diff != "" {
					t.Errorf("header #%d (-want +got):\n%s", i+1, diff)
				}
				if got, err := io.ReadAll(r); string(got) != test.want[i].data || err != nil {
					t.Errorf("io.ReadAll(r) #%d = %q, %v; want %q, <nil>", i+1, got, err, test.want[i].data)
				}
			}

			got, err := r.Next()
			if err != io.EOF {
				t.Errorf("r.Next() #%d = %+v, %v; want _, %v", len(test.want), got, err, io.EOF)
			}
		})
	}

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
			t.Errorf("r.Next() = _, %v; want _, <!EOF>", err)
		} else {
			t.Logf("r.Next() = _, %v", err)
		}
	})
}

func BenchmarkReader(b *testing.B) {
	data, err := os.ReadFile(filepath.Join("testdata", "mini-drv.nar"))
	if err != nil {
		b.Fatal(err)
	}
	r := bytes.NewReader(nil)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Reset(data)
		nr := NewReader(r)
		for {
			if _, err := nr.Next(); err == io.EOF {
				break
			} else if err != nil {
				b.Fatal(err)
			}
			if _, err := io.Copy(io.Discard, nr); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func FuzzReader(f *testing.F) {
	listing, err := os.ReadDir("testdata")
	if err != nil {
		f.Fatal(err)
	}
	for _, ent := range listing {
		if name := ent.Name(); strings.HasSuffix(name, ".nar") && !strings.HasPrefix(name, ".") {
			data, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				f.Fatal(err)
			}
			f.Add(data)
		}
	}

	f.Fuzz(func(t *testing.T, in []byte) {
		r := NewReader(bytes.NewReader(in))
		for {
			if _, err := r.Next(); err != nil {
				t.Log("Stopped from error:", err)
				return
			}
			io.Copy(io.Discard, r)
		}
	})
}
