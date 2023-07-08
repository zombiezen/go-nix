package nar

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestReader(t *testing.T) {
	type entry struct {
		header *Header
		data   string
	}
	tests := []struct {
		name     string
		dataFile string
		want     []entry
	}{
		{
			name:     "EmptyFile",
			dataFile: "empty-file.nar",
			want: []entry{
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
			want: []entry{
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
			want: []entry{
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
			want: []entry{
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
			want: []entry{
				{
					header: &Header{
						Mode:       fs.ModeSymlink | 0o777,
						LinkTarget: "foo/bar/baz",
					},
				},
			},
		},
		{
			name:     "Tree",
			dataFile: "mini-drv.nar",
			want: []entry{
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
						Size: 45,
					},
					data: "#!/bin/sh\n" +
						`cat "$(dirname "$0")/../hello.txt"` + "\n",
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

	for _, test := range tests {
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
