package nar

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestFS(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "mini-drv.nar"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		ls, err := List(f)
		if err != nil {
			t.Fatal(err)
		}
		fsys, err := NewFS(f, ls)
		if err != nil {
			t.Fatal(err)
		}

		if err := fstest.TestFS(fsys, "a.txt", "bin/hello.sh", "hello.txt"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Symlinks", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "nar_1094wph9z4nwlgvsd53abfz8i117ykiv5dwnq9nnhz846s7xqd7d.nar"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		ls, err := List(f)
		if err != nil {
			t.Fatal(err)
		}
		fsys, err := NewFS(f, ls)
		if err != nil {
			t.Fatal(err)
		}

		if got, err := fsys.ReadLink("sbin"); got != "bin" || err != nil {
			t.Errorf("fsys.ReadLink(%q) = %q, %v; want %q, <nil>", "sbin", got, err, "bin")
		}

		// Both directory and final name are symlinks.
		{
			const path = "sbin/domainname"
			got, err := fsys.Stat(path)
			if err != nil {
				t.Errorf("fsys.Stat(%q): %v", path, err)
			} else {
				if want := int64(17704); got.Size() != want {
					t.Errorf("fsys.Stat(%q).Size() = %d; want %d", path, got.Size(), want)
				}
				if want := fs.FileMode(0o555); got.Mode() != want {
					t.Errorf("fsys.Stat(%q).Mode() = %v; want %v", path, got.Mode(), want)
				}
			}
		}
	})
}
