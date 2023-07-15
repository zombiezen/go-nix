package nar

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestList(t *testing.T) {
	for _, test := range narTests {
		if test.err {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			f, err := os.Open(filepath.Join("testdata", test.dataFile))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			got, err := List(f)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(&test.wantList, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("-want +got:\n%s", diff)
			}
		})
	}
}

const testListingJSON = `
{
  "version": 1,
  "root": {
    "type": "directory",
    "entries": {
      "bin": {
        "type": "directory",
        "entries": {
          "curl": {
            "type": "regular",
            "size": 182520,
            "executable": true,
            "narOffset": 400
          }
        }
      },
			"sbin": {
				"type": "symlink",
				"target": "bin"
			}
    }
  }
}
`

func wantListing() *Listing {
	return &Listing{
		Root: ListingNode{
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
						"curl": {Header: Header{
							Path:          "bin/curl",
							Mode:          0o555,
							Size:          182520,
							ContentOffset: 400,
						}},
					},
				},
				"sbin": {Header: Header{
					Path:       "sbin",
					Mode:       fs.ModeSymlink | 0o777,
					LinkTarget: "bin",
				}},
			},
		},
	}
}

func TestListingMarshalJSON(t *testing.T) {
	gotJSON, err := json.Marshal(wantListing())
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseJSONTestValue(gotJSON)
	if err != nil {
		t.Fatal(err)
	}
	want, err := parseJSONTestValue([]byte(testListingJSON))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("-want +got:\n%s", diff)
	}
}

func TestListingUnmarshalJSON(t *testing.T) {
	got := new(Listing)
	if err := json.Unmarshal([]byte(testListingJSON), &got); err != nil {
		t.Error(err)
	}
	want := wantListing()
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("-want +got:\n%s", diff)
	}
}

func parseJSONTestValue(data []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}
