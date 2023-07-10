package nar_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/nix-community/go-nix/pkg/nar"
)

type testEntry struct {
	header *Header
	data   string
}

const helloWorld = "Hello, World!\n"

const helloWorldScriptData = "#!/bin/sh\necho 'Hello, World!'\n"

const miniDRVScriptData = "#!/bin/sh\n" +
	`cat "$(dirname "$0")/../hello.txt"` + "\n"

var narTests = []struct {
	name           string
	dataFile       string
	want           []testEntry
	ignoreContents bool
	err            bool
}{
	{
		name:     "EmptyFile",
		dataFile: "empty-file.nar",
		want: []testEntry{
			{
				header: &Header{
					Type: TypeRegular,
					Path: "/",
					Size: 0,
				},
			},
		},
	},
	{
		name:     "OneByteFile",
		dataFile: "1byte-regular.nar",
		want: []testEntry{
			{
				header: &Header{
					Type: TypeRegular,
					Path: "/",
					Size: 1,
				},
				data: "\x01",
			},
		},
	},
	{
		name:     "EmptyDirectory",
		dataFile: "empty-directory.nar",
		want: []testEntry{
			{
				header: &Header{
					Type: TypeDirectory,
					Path: "/",
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
					Type: TypeRegular,
					Path: "/",
					Size: int64(len(helloWorld)),
				},
				data: helloWorld,
			},
		},
	},
	{
		name:     "Script",
		dataFile: "hello-script.nar",
		want: []testEntry{
			{
				header: &Header{
					Type:       TypeRegular,
					Path:       "/",
					Executable: true,
					Size:       int64(len(helloWorldScriptData)),
				},
				data: helloWorldScriptData,
			},
		},
	},
	{
		name:     "Symlink",
		dataFile: "symlink.nar",
		want: []testEntry{
			{
				header: &Header{
					Type:       TypeSymlink,
					Path:       "/",
					LinkTarget: "/nix/store/somewhereelse",
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
					Type: TypeDirectory,
					Path: "/",
				},
			},
			{
				header: &Header{
					Type: TypeRegular,
					Path: "/a.txt",
					Size: 4,
				},
				data: "AAA\n",
			},
			{
				header: &Header{
					Type: TypeDirectory,
					Path: "/bin",
				},
			},
			{
				header: &Header{
					Type:       TypeRegular,
					Path:       "/bin/hello.sh",
					Executable: true,
					Size:       int64(len(miniDRVScriptData)),
				},
				data: miniDRVScriptData,
			},
			{
				header: &Header{
					Type: TypeRegular,
					Path: "/hello.txt",
					Size: int64(len(helloWorld)),
				},
				data: helloWorld,
			},
		},
	},
	{
		name:     "NestedDirAndCommonPrefix",
		dataFile: "nested-dir-and-common-prefix.nar",
		want: []testEntry{
			{
				header: &Header{
					Path: "/",
					Type: TypeDirectory,
				},
			},
			{
				header: &Header{
					Path: "/foo",
					Type: TypeDirectory,
				},
			},
			{
				header: &Header{
					Path:       "/foo/b",
					Type:       TypeSymlink,
					LinkTarget: "foo",
				},
			},
			{
				header: &Header{
					Path:       "/foo-a",
					Type:       TypeSymlink,
					LinkTarget: "foo",
				},
			},
		},
	},
	{
		name:           "SmokeTest",
		dataFile:       "nar_1094wph9z4nwlgvsd53abfz8i117ykiv5dwnq9nnhz846s7xqd7d.nar",
		ignoreContents: true,
		want: []testEntry{
			{header: &Header{Type: TypeDirectory, Path: "/"}},
			{header: &Header{Type: TypeDirectory, Path: "/bin"}},
			{header: &Header{
				Type:       TypeRegular,
				Path:       "/bin/arp",
				Executable: true,
				Size:       55288,
			}},
			{header: &Header{
				Type:       TypeSymlink,
				Path:       "/bin/dnsdomainname",
				LinkTarget: "hostname",
			}},
			{header: &Header{
				Type:       TypeSymlink,
				Path:       "/bin/domainname",
				LinkTarget: "hostname",
			}},
			{header: &Header{
				Type:       TypeRegular,
				Path:       "/bin/hostname",
				Executable: true,
				Size:       17704,
			}},
			{header: &Header{
				Type:       TypeRegular,
				Path:       "/bin/ifconfig",
				Executable: true,
				Size:       72576,
			}},
			{header: &Header{
				Type:       TypeRegular,
				Path:       "/bin/nameif",
				Executable: true,
				Size:       18776,
			}},
			{header: &Header{
				Type:       TypeRegular,
				Path:       "/bin/netstat",
				Executable: true,
				Size:       131784,
			}},
			{header: &Header{
				Type:       TypeSymlink,
				Path:       "/bin/nisdomainname",
				LinkTarget: "hostname",
			}},
			{header: &Header{
				Type:       TypeRegular,
				Path:       "/bin/plipconfig",
				Executable: true,
				Size:       13160,
			}},
			{header: &Header{
				Type:       TypeRegular,
				Path:       "/bin/rarp",
				Executable: true,
				Size:       30384,
			}},
			{header: &Header{
				Type:       TypeRegular,
				Path:       "/bin/route",
				Executable: true,
				Size:       61928,
			}},
			{header: &Header{
				Type:       TypeRegular,
				Path:       "/bin/slattach",
				Executable: true,
				Size:       35672,
			}},
			{header: &Header{
				Type:       TypeSymlink,
				Path:       "/bin/ypdomainname",
				LinkTarget: "hostname",
			}},
			{header: &Header{
				Type:       TypeSymlink,
				Path:       "/sbin",
				LinkTarget: "bin",
			}},
			{header: &Header{
				Type: TypeDirectory,
				Path: "/share",
			}},
			{header: &Header{
				Type: TypeDirectory,
				Path: "/share/man",
			}},
			{header: &Header{
				Type: TypeDirectory,
				Path: "/share/man/man1",
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man1/dnsdomainname.1.gz",
				Size: 40,
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man1/domainname.1.gz",
				Size: 40,
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man1/hostname.1.gz",
				Size: 1660,
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man1/nisdomainname.1.gz",
				Size: 40,
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man1/ypdomainname.1.gz",
				Size: 40,
			}},
			{header: &Header{
				Type: TypeDirectory,
				Path: "/share/man/man5",
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man5/ethers.5.gz",
				Size: 563,
			}},
			{header: &Header{
				Type: TypeDirectory,
				Path: "/share/man/man8",
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man8/arp.8.gz",
				Size: 2464,
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man8/ifconfig.8.gz",
				Size: 3382,
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man8/nameif.8.gz",
				Size: 523,
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man8/netstat.8.gz",
				Size: 4284,
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man8/plipconfig.8.gz",
				Size: 889,
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man8/rarp.8.gz",
				Size: 1198,
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man8/route.8.gz",
				Size: 3525,
			}},
			{header: &Header{
				Type: TypeRegular,
				Path: "/share/man/man8/slattach.8.gz",
				Size: 1441,
			}},
		},
	},
	{
		name:     "OnlyMagic",
		dataFile: "only-magic.nar",
		err:      true,
	},
	{
		name:     "InvalidOrder",
		dataFile: "invalid-order.nar",
		want: []testEntry{
			{
				header: &Header{
					Type: TypeDirectory,
					Path: "/",
				},
			},
			{
				header: &Header{
					Type: TypeDirectory,
					Path: "/b",
				},
			},
		},
		err: true,
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
			r, err := NewReader(f)
			switch {
			case err != nil && (!test.err || len(test.want) > 0):
				t.Fatal("NewReader:", err)
			case err != nil && test.err && len(test.want) == 0:
				t.Log("NewReader:", err)
				return
			}

			for i := range test.want {
				gotHeader, err := r.Next()
				if err != nil {
					t.Fatalf("r.Next() #%d: %v", i+1, err)
				}
				if diff := cmp.Diff(test.want[i].header, gotHeader); diff != "" {
					t.Errorf("header #%d (-want +got):\n%s", i+1, diff)
				}
				if !test.ignoreContents {
					if got, err := io.ReadAll(r); string(got) != test.want[i].data || err != nil {
						t.Errorf("io.ReadAll(r) #%d = %q, %v; want %q, <nil>", i+1, got, err, test.want[i].data)
					}
				}
			}

			got, err := r.Next()
			if err == nil || !test.err && err != io.EOF || test.err && err == io.EOF {
				errString := io.EOF.Error()
				if test.err {
					errString = "<non-EOF error>"
				}
				t.Errorf("r.Next() #%d = %+v, %v; want _, %s", len(test.want), got, err, errString)
			}
		})
	}
}

func BenchmarkReader(b *testing.B) {
	data, err := os.ReadFile(filepath.Join("testdata", "nar_1094wph9z4nwlgvsd53abfz8i117ykiv5dwnq9nnhz846s7xqd7d.nar"))
	if err != nil {
		b.Fatal(err)
	}
	r := bytes.NewReader(nil)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Reset(data)
		nr, err := NewReader(r)
		if err != nil {
			b.Fatal(err)
		}
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
		nr, err := NewReader(bytes.NewReader(in))
		if err != nil {
			t.Log("NewReader error:", err)
			return
		}
		for {
			if _, err := nr.Next(); err != nil {
				t.Log("Stopped from error:", err)
				return
			}
			io.Copy(io.Discard, nr)
		}
	})
}
