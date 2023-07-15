package nar

import (
	"bytes"
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

const helloWorld = "Hello, World!\n"

const helloWorldScriptData = "#!/bin/sh\necho 'Hello, World!'\n"

const miniDRVScriptData = "#!/bin/sh\n" +
	`cat "$(dirname "$0")/../hello.txt"` + "\n"

var narTests = []struct {
	name           string
	dataFile       string
	want           []testEntry
	wantList       Listing
	ignoreContents bool
	err            bool
}{
	{
		name:     "EmptyFile",
		dataFile: "empty-file.nar",
		want: []testEntry{
			{
				header: &Header{
					Path:          "",
					Mode:          0o444,
					Size:          0,
					ContentOffset: 96,
				},
			},
		},
		wantList: Listing{Root: ListingNode{
			Header: Header{
				Mode:          0o444,
				Size:          0,
				ContentOffset: 96,
			},
		}},
	},
	{
		name:     "OneByteFile",
		dataFile: "1byte-regular.nar",
		want: []testEntry{
			{
				header: &Header{
					Path:          "",
					Mode:          0o444,
					Size:          1,
					ContentOffset: 96,
				},
				data: "\x01",
			},
		},
		wantList: Listing{Root: ListingNode{
			Header: Header{
				Mode:          0o444,
				Size:          1,
				ContentOffset: 96,
			},
		}},
	},
	{
		name:     "EmptyDirectory",
		dataFile: "empty-directory.nar",
		want: []testEntry{
			{
				header: &Header{
					Mode: fs.ModeDir | 0o555,
				},
			},
		},
		wantList: Listing{Root: ListingNode{
			Header: Header{
				Mode: fs.ModeDir | 0o555,
			},
		}},
	},
	{
		name:     "TextFile",
		dataFile: "hello-world.nar",
		want: []testEntry{
			{
				header: &Header{
					Mode:          0o444,
					Size:          int64(len(helloWorld)),
					ContentOffset: 96,
				},
				data: helloWorld,
			},
		},
		wantList: Listing{Root: ListingNode{
			Header: Header{
				Mode:          0o444,
				Size:          int64(len(helloWorld)),
				ContentOffset: 96,
			},
		}},
	},
	{
		name:     "Script",
		dataFile: "hello-script.nar",
		want: []testEntry{
			{
				header: &Header{
					Mode:          0o555,
					Size:          int64(len(helloWorldScriptData)),
					ContentOffset: 128,
				},
				data: helloWorldScriptData,
			},
		},
		wantList: Listing{Root: ListingNode{
			Header: Header{
				Mode:          0o555,
				Size:          int64(len(helloWorldScriptData)),
				ContentOffset: 128,
			},
		}},
	},
	{
		name:     "Symlink",
		dataFile: "symlink.nar",
		want: []testEntry{
			{
				header: &Header{
					Mode:       fs.ModeSymlink | 0o777,
					LinkTarget: "/nix/store/somewhereelse",
				},
			},
		},
		wantList: Listing{Root: ListingNode{
			Header: Header{
				Mode:       fs.ModeSymlink | 0o777,
				LinkTarget: "/nix/store/somewhereelse",
			},
		}},
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
					Path:          "a.txt",
					Mode:          0o444,
					Size:          4,
					ContentOffset: 232,
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
					Path:          "bin/hello.sh",
					Mode:          0o555,
					Size:          int64(len(miniDRVScriptData)),
					ContentOffset: 592,
				},
				data: miniDRVScriptData,
			},
			{
				header: &Header{
					Path:          "hello.txt",
					Mode:          0o444,
					Size:          int64(len(helloWorld)),
					ContentOffset: 864,
				},
				data: helloWorld,
			},
		},
		wantList: Listing{Root: ListingNode{
			Header: Header{
				Mode: fs.ModeDir | 0o555,
			},
			Entries: map[string]*ListingNode{
				"a.txt": {Header: Header{
					Path:          "a.txt",
					Mode:          0o444,
					Size:          4,
					ContentOffset: 232,
				}},
				"bin": {
					Header: Header{
						Path: "bin",
						Mode: fs.ModeDir | 0o555,
					},
					Entries: map[string]*ListingNode{
						"hello.sh": {Header: Header{
							Path:          "bin/hello.sh",
							Mode:          0o555,
							Size:          int64(len(miniDRVScriptData)),
							ContentOffset: 592,
						}},
					},
				},
				"hello.txt": {Header: Header{
					Path:          "hello.txt",
					Mode:          0o444,
					Size:          int64(len(helloWorld)),
					ContentOffset: 864,
				}},
			},
		}},
	},
	{
		name:     "NestedDirAndCommonPrefix",
		dataFile: "nested-dir-and-common-prefix.nar",
		want: []testEntry{
			{
				header: &Header{
					Mode: fs.ModeDir | 0o555,
				},
			},
			{
				header: &Header{
					Path: "foo",
					Mode: fs.ModeDir | 0o555,
				},
			},
			{
				header: &Header{
					Path:       "foo/b",
					Mode:       fs.ModeSymlink | 0o777,
					LinkTarget: "foo",
				},
			},
			{
				header: &Header{
					Path:       "foo-a",
					Mode:       fs.ModeSymlink | 0o777,
					LinkTarget: "foo",
				},
			},
		},
		wantList: Listing{Root: ListingNode{
			Header: Header{
				Mode: fs.ModeDir | 0o555,
			},
			Entries: map[string]*ListingNode{
				"foo": {
					Header: Header{
						Path: "foo",
						Mode: fs.ModeDir | 0o555,
					},
					Entries: map[string]*ListingNode{
						"b": {Header: Header{
							Path:       "foo/b",
							Mode:       fs.ModeSymlink | 0o777,
							LinkTarget: "foo",
						}},
					},
				},
				"foo-a": {Header: Header{
					Path:       "foo-a",
					Mode:       fs.ModeSymlink | 0o777,
					LinkTarget: "foo",
				}},
			},
		}},
	},
	{
		name:           "SmokeTest",
		dataFile:       "nar_1094wph9z4nwlgvsd53abfz8i117ykiv5dwnq9nnhz846s7xqd7d.nar",
		ignoreContents: true,
		want: []testEntry{
			{header: &Header{
				Mode: fs.ModeDir | 0o555,
			}},
			{header: &Header{
				Path: "bin",
				Mode: fs.ModeDir | 0o555,
			}},
			{header: &Header{
				Path:          "bin/arp",
				Mode:          0o555,
				Size:          55288,
				ContentOffset: 400,
			}},
			{header: &Header{
				Path:       "bin/dnsdomainname",
				Mode:       fs.ModeSymlink | 0o777,
				LinkTarget: "hostname",
			}},
			{header: &Header{
				Path:       "bin/domainname",
				Mode:       fs.ModeSymlink | 0o777,
				LinkTarget: "hostname",
			}},
			{header: &Header{
				Path:          "bin/hostname",
				Mode:          0o555,
				Size:          17704,
				ContentOffset: 56304,
			}},
			{header: &Header{
				Path:          "bin/ifconfig",
				Mode:          0o555,
				Size:          72576,
				ContentOffset: 74224,
			}},
			{header: &Header{
				Path:          "bin/nameif",
				Mode:          0o555,
				Size:          18776,
				ContentOffset: 147016,
			}},
			{header: &Header{
				Path:          "bin/netstat",
				Mode:          0o555,
				Size:          131784,
				ContentOffset: 166008,
			}},
			{header: &Header{
				Path:       "bin/nisdomainname",
				Mode:       fs.ModeSymlink | 0o777,
				LinkTarget: "hostname",
			}},
			{header: &Header{
				Path:          "bin/plipconfig",
				Mode:          0o555,
				Size:          13160,
				ContentOffset: 298216,
			}},
			{header: &Header{
				Path:          "bin/rarp",
				Mode:          0o555,
				Size:          30384,
				ContentOffset: 311592,
			}},
			{header: &Header{
				Path:          "bin/route",
				Mode:          0o555,
				Size:          61928,
				ContentOffset: 342192,
			}},
			{header: &Header{
				Path:          "bin/slattach",
				Mode:          0o555,
				Size:          35672,
				ContentOffset: 404336,
			}},
			{header: &Header{
				Path:       "bin/ypdomainname",
				Mode:       fs.ModeSymlink | 0o777,
				LinkTarget: "hostname",
			}},
			{header: &Header{
				Path:       "sbin",
				Mode:       fs.ModeSymlink | 0o777,
				LinkTarget: "bin",
			}},
			{header: &Header{
				Path: "share",
				Mode: fs.ModeDir | 0o555,
			}},
			{header: &Header{
				Path: "share/man",
				Mode: fs.ModeDir | 0o555,
			}},
			{header: &Header{
				Path: "share/man/man1",
				Mode: fs.ModeDir | 0o555,
			}},
			{header: &Header{
				Path:          "share/man/man1/dnsdomainname.1.gz",
				Mode:          0o444,
				Size:          40,
				ContentOffset: 441040,
			}},
			{header: &Header{
				Path:          "share/man/man1/domainname.1.gz",
				Mode:          0o444,
				Size:          40,
				ContentOffset: 441272,
			}},
			{header: &Header{
				Path:          "share/man/man1/hostname.1.gz",
				Mode:          0o444,
				Size:          1660,
				ContentOffset: 441504,
			}},
			{header: &Header{
				Path:          "share/man/man1/nisdomainname.1.gz",
				Mode:          0o444,
				Size:          40,
				ContentOffset: 443368,
			}},
			{header: &Header{
				Path:          "share/man/man1/ypdomainname.1.gz",
				Mode:          0o444,
				Size:          40,
				ContentOffset: 443608,
			}},
			{header: &Header{
				Path: "share/man/man5",
				Mode: fs.ModeDir | 0o555,
			}},
			{header: &Header{
				Path:          "share/man/man5/ethers.5.gz",
				Mode:          0o444,
				Size:          563,
				ContentOffset: 444008,
			}},
			{header: &Header{
				Path: "share/man/man8",
				Mode: fs.ModeDir | 0o555,
			}},
			{header: &Header{
				Path:          "share/man/man8/arp.8.gz",
				Mode:          0o444,
				Size:          2464,
				ContentOffset: 444928,
			}},
			{header: &Header{
				Path:          "share/man/man8/ifconfig.8.gz",
				Mode:          0o444,
				Size:          3382,
				ContentOffset: 447584,
			}},
			{header: &Header{
				Path:          "share/man/man8/nameif.8.gz",
				Mode:          0o444,
				Size:          523,
				ContentOffset: 451160,
			}},
			{header: &Header{
				Path:          "share/man/man8/netstat.8.gz",
				Mode:          0o444,
				Size:          4284,
				ContentOffset: 451880,
			}},
			{header: &Header{
				Path:          "share/man/man8/plipconfig.8.gz",
				Mode:          0o444,
				Size:          889,
				ContentOffset: 456360,
			}},
			{header: &Header{
				Path:          "share/man/man8/rarp.8.gz",
				Mode:          0o444,
				Size:          1198,
				ContentOffset: 457448,
			}},
			{header: &Header{
				Path:          "share/man/man8/route.8.gz",
				Mode:          0o444,
				Size:          3525,
				ContentOffset: 458840,
			}},
			{header: &Header{
				Path:          "share/man/man8/slattach.8.gz",
				Mode:          0o444,
				Size:          1441,
				ContentOffset: 462560,
			}},
		},
		wantList: Listing{Root: ListingNode{
			Header: Header{
				Mode: fs.ModeDir | 0o555,
			},
			Entries: map[string]*ListingNode{
				"bin": {
					Header: Header{
						Path: "bin",
						Mode: fs.ModeDir | 0o555,
					},
					Entries: map[string]*ListingNode{
						"arp": {Header: Header{
							Path:          "bin/arp",
							Mode:          0o555,
							Size:          55288,
							ContentOffset: 400,
						}},
						"dnsdomainname": {Header: Header{
							Path:       "bin/dnsdomainname",
							Mode:       fs.ModeSymlink | 0o777,
							LinkTarget: "hostname",
						}},
						"domainname": {Header: Header{
							Path:       "bin/domainname",
							Mode:       fs.ModeSymlink | 0o777,
							LinkTarget: "hostname",
						}},
						"hostname": {Header: Header{
							Path:          "bin/hostname",
							Mode:          0o555,
							Size:          17704,
							ContentOffset: 56304,
						}},
						"ifconfig": {Header: Header{
							Path:          "bin/ifconfig",
							Mode:          0o555,
							Size:          72576,
							ContentOffset: 74224,
						}},
						"nameif": {Header: Header{
							Path:          "bin/nameif",
							Mode:          0o555,
							Size:          18776,
							ContentOffset: 147016,
						}},
						"netstat": {Header: Header{
							Path:          "bin/netstat",
							Mode:          0o555,
							Size:          131784,
							ContentOffset: 166008,
						}},
						"nisdomainname": {Header: Header{
							Path:       "bin/nisdomainname",
							Mode:       fs.ModeSymlink | 0o777,
							LinkTarget: "hostname",
						}},
						"plipconfig": {Header: Header{
							Path:          "bin/plipconfig",
							Mode:          0o555,
							Size:          13160,
							ContentOffset: 298216,
						}},
						"rarp": {Header: Header{
							Path:          "bin/rarp",
							Mode:          0o555,
							Size:          30384,
							ContentOffset: 311592,
						}},
						"route": {Header: Header{
							Path:          "bin/route",
							Mode:          0o555,
							Size:          61928,
							ContentOffset: 342192,
						}},
						"slattach": {Header: Header{
							Path:          "bin/slattach",
							Mode:          0o555,
							Size:          35672,
							ContentOffset: 404336,
						}},
						"ypdomainname": {Header: Header{
							Path:       "bin/ypdomainname",
							Mode:       fs.ModeSymlink | 0o777,
							LinkTarget: "hostname",
						}},
					},
				},
				"sbin": {Header: Header{
					Path:       "sbin",
					Mode:       fs.ModeSymlink | 0o777,
					LinkTarget: "bin",
				}},
				"share": {
					Header: Header{
						Path: "share",
						Mode: fs.ModeDir | 0o555,
					},
					Entries: map[string]*ListingNode{
						"man": {
							Header: Header{
								Path: "share/man",
								Mode: fs.ModeDir | 0o555,
							},
							Entries: map[string]*ListingNode{
								"man1": {
									Header: Header{
										Path: "share/man/man1",
										Mode: fs.ModeDir | 0o555,
									},
									Entries: map[string]*ListingNode{
										"dnsdomainname.1.gz": {Header: Header{
											Path:          "share/man/man1/dnsdomainname.1.gz",
											Mode:          0o444,
											Size:          40,
											ContentOffset: 441040,
										}},
										"domainname.1.gz": {Header: Header{
											Path:          "share/man/man1/domainname.1.gz",
											Mode:          0o444,
											Size:          40,
											ContentOffset: 441272,
										}},
										"hostname.1.gz": {Header: Header{
											Path:          "share/man/man1/hostname.1.gz",
											Mode:          0o444,
											Size:          1660,
											ContentOffset: 441504,
										}},
										"nisdomainname.1.gz": {Header: Header{
											Path:          "share/man/man1/nisdomainname.1.gz",
											Mode:          0o444,
											Size:          40,
											ContentOffset: 443368,
										}},
										"ypdomainname.1.gz": {Header: Header{
											Path:          "share/man/man1/ypdomainname.1.gz",
											Mode:          0o444,
											Size:          40,
											ContentOffset: 443608,
										}},
									},
								},
								"man5": {
									Header: Header{
										Path: "share/man/man5",
										Mode: fs.ModeDir | 0o555,
									},
									Entries: map[string]*ListingNode{
										"ethers.5.gz": {Header: Header{
											Path:          "share/man/man5/ethers.5.gz",
											Mode:          0o444,
											Size:          563,
											ContentOffset: 444008,
										}},
									},
								},
								"man8": {
									Header: Header{
										Path: "share/man/man8",
										Mode: fs.ModeDir | 0o555,
									},
									Entries: map[string]*ListingNode{
										"arp.8.gz": {Header: Header{
											Path:          "share/man/man8/arp.8.gz",
											Mode:          0o444,
											Size:          2464,
											ContentOffset: 444928,
										}},
										"ifconfig.8.gz": {Header: Header{
											Path:          "share/man/man8/ifconfig.8.gz",
											Mode:          0o444,
											Size:          3382,
											ContentOffset: 447584,
										}},
										"nameif.8.gz": {Header: Header{
											Path:          "share/man/man8/nameif.8.gz",
											Mode:          0o444,
											Size:          523,
											ContentOffset: 451160,
										}},
										"netstat.8.gz": {Header: Header{
											Path:          "share/man/man8/netstat.8.gz",
											Mode:          0o444,
											Size:          4284,
											ContentOffset: 451880,
										}},
										"plipconfig.8.gz": {Header: Header{
											Path:          "share/man/man8/plipconfig.8.gz",
											Mode:          0o444,
											Size:          889,
											ContentOffset: 456360,
										}},
										"rarp.8.gz": {Header: Header{
											Path:          "share/man/man8/rarp.8.gz",
											Mode:          0o444,
											Size:          1198,
											ContentOffset: 457448,
										}},
										"route.8.gz": {Header: Header{
											Path:          "share/man/man8/route.8.gz",
											Mode:          0o444,
											Size:          3525,
											ContentOffset: 458840,
										}},
										"slattach.8.gz": {Header: Header{
											Path:          "share/man/man8/slattach.8.gz",
											Mode:          0o444,
											Size:          1441,
											ContentOffset: 462560,
										}},
									},
								},
							},
						},
					},
				},
			},
		}},
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
					Path: "",
					Mode: fs.ModeDir | 0o555,
				},
			},
			{
				header: &Header{
					Path: "b",
					Mode: fs.ModeDir | 0o555,
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
			nr := NewReader(f)

			for i := range test.want {
				gotHeader, err := nr.Next()
				if err != nil {
					t.Fatalf("r.Next() #%d: %v", i+1, err)
				}
				if diff := cmp.Diff(test.want[i].header, gotHeader); diff != "" {
					t.Errorf("header #%d (-want +got):\n%s", i+1, diff)
				}
				if !test.ignoreContents {
					if got, err := io.ReadAll(nr); string(got) != test.want[i].data || err != nil {
						t.Errorf("io.ReadAll(r) #%d = %q, %v; want %q, <nil>", i+1, got, err, test.want[i].data)
					}
				}
			}

			got, err := nr.Next()
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
		nr := NewReader(bytes.NewReader(in))
		for {
			if _, err := nr.Next(); err != nil {
				t.Log("Stopped from error:", err)
				return
			}
			io.Copy(io.Discard, nr)
		}
	})
}
