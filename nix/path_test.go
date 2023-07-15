package nix

import (
	slashpath "path"
	"strings"
	"testing"
)

var storePathTests = []struct {
	path string
	err  bool

	dir          StoreDirectory
	base         string
	digestPart   string
	namePart     string
	isDerivation bool
}{
	{
		path: "",
		err:  true,
	},
	{
		path: "foo",
		err:  true,
	},
	{
		path: "foo/ffffffffffffffffffffffffffffffff-x",
		err:  true,
	},
	{
		path: "/nix/store",
		err:  true,
	},
	{
		path: "/nix/store/ffffffffffffffffffffffffffffffff",
		err:  true,
	},
	{
		path: "/nix/store/ffffffffffffffffffffffffffffffff-",
		err:  true,
	},
	{
		path: "/nix/store/ffffffffffffffffffffffffffffffff_x",
		err:  true,
	},
	{
		path: "/nix/store/ffffffffffffffffffffffffffffffff-" + strings.Repeat("x", 212),
		err:  true,
	},
	{
		path: "/nix/store/ffffffffffffffffffffffffffffffff-foo@bar",
		err:  true,
	},
	{
		path: "/nix/store/eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee-x",
		err:  true,
	},
	{
		path: "/nix/store/00bgd045z0d4icpbc2yy-net-tools-1.60_p20170221182432",
		err:  true,
	},
	{
		path: "/nix/store/00bgd045z0d4icpbc2yyz4gx48aku4la-net-tools-1.60_p20170221182432",
		err:  true,
	},
	{
		path:       "/nix/store/ffffffffffffffffffffffffffffffff-x",
		dir:        "/nix/store",
		base:       "ffffffffffffffffffffffffffffffff-x",
		digestPart: "ffffffffffffffffffffffffffffffff",
		namePart:   "x",
	},
	{
		path:       "/nix/store/ffffffffffffffffffffffffffffffff-x/",
		dir:        "/nix/store",
		base:       "ffffffffffffffffffffffffffffffff-x",
		digestPart: "ffffffffffffffffffffffffffffffff",
		namePart:   "x",
	},
	{
		path:       "/nix/store/foo/../ffffffffffffffffffffffffffffffff-x",
		dir:        "/nix/store",
		base:       "ffffffffffffffffffffffffffffffff-x",
		digestPart: "ffffffffffffffffffffffffffffffff",
		namePart:   "x",
	},
	{
		path:       "/nix/store/s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1",
		dir:        "/nix/store",
		base:       "s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1",
		digestPart: "s66mzxpvicwk07gjbjfw9izjfa797vsw",
		namePart:   "hello-2.12.1",
	},
	{
		path:         "/nix/store/ib3sh3pcz10wsmavxvkdbayhqivbghlq-hello-2.12.1.drv",
		dir:          "/nix/store",
		base:         "ib3sh3pcz10wsmavxvkdbayhqivbghlq-hello-2.12.1.drv",
		digestPart:   "ib3sh3pcz10wsmavxvkdbayhqivbghlq",
		namePart:     "hello-2.12.1.drv",
		isDerivation: true,
	},
	{
		path:       "/nix/store/00bgd045z0d4icpbc2yyz4gx48ak44la-net-tools-1.60_p20170221182432",
		dir:        "/nix/store",
		base:       "00bgd045z0d4icpbc2yyz4gx48ak44la-net-tools-1.60_p20170221182432",
		digestPart: "00bgd045z0d4icpbc2yyz4gx48ak44la",
		namePart:   "net-tools-1.60_p20170221182432",
	},
}

func TestParseStorePath(t *testing.T) {
	for _, test := range storePathTests {
		storePath, err := ParseStorePath(test.path)
		if test.err {
			if err == nil {
				t.Errorf("ParseStorePath(%q) = %q, <nil>; want _, <error>", test.path, storePath)
			}
			continue
		}
		if want := StorePath(slashpath.Clean(test.path)); storePath != want || err != nil {
			t.Errorf("ParseStorePath(%q) = %q, %v; want %q, <nil>", test.path, storePath, err, want)
		}
		if err != nil {
			continue
		}
		if got, want := storePath.Dir(), test.dir; got != want {
			t.Errorf("ParseStorePath(%q).Dir() = %q; want %q", test.path, got, want)
		}
		if got, want := storePath.Base(), test.base; got != want {
			t.Errorf("ParseStorePath(%q).Base() = %q; want %q", test.path, got, want)
		}
		if got, want := storePath.Digest(), test.digestPart; got != want {
			t.Errorf("ParseStorePath(%q).Digest() = %q; want %q", test.path, got, want)
		}
		if got, want := storePath.Name(), test.namePart; got != want {
			t.Errorf("ParseStorePath(%q).Name() = %q; want %q", test.path, got, want)
		}
		if got, want := storePath.IsDerivation(), test.isDerivation; got != want {
			t.Errorf("ParseStorePath(%q).IsDerivation() = %t; want %t", test.path, got, want)
		}
	}
}

func TestStoreDirectoryObject(t *testing.T) {
	for _, test := range storePathTests {
		if test.err {
			continue
		}
		got, err := test.dir.Object(test.base)
		want := StorePath(slashpath.Clean(test.path))
		if got != want || err != nil {
			t.Errorf("StoreDirectory(%q).Object(%q) = %q, %v; want %q, <nil>",
				test.dir, test.base, got, err, want)
		}
	}

	badObjectNames := []string{
		"",
		".",
		"..",
		"foo/bar",
	}
	for _, name := range badObjectNames {
		got, err := DefaultStoreDirectory.Object(name)
		if err == nil {
			t.Errorf("StoreDirectory(%q).Object(%q) = %q, <nil>; want _, <error>",
				DefaultStoreDirectory, name, got)
		}
	}
}

func TestStoreDirectoryParsePath(t *testing.T) {
	type parsePathTest struct {
		dir  StoreDirectory
		path string

		want StorePath
		sub  string
		err  bool
	}
	tests := []parsePathTest{
		{
			dir:  "/nix/store",
			path: "/nix/store/s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1/bin/hello",

			want: "/nix/store/s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1",
			sub:  "bin/hello",
		},
		{
			dir:  "/nix/store",
			path: "",
			err:  true,
		},
		{
			dir:  "/nix/store",
			path: "",
			err:  true,
		},
		{
			dir:  "/nix/store",
			path: "nix/store",
			err:  true,
		},
		{
			dir:  "foo",
			path: "foo/ffffffffffffffffffffffffffffffff-x",
			err:  true,
		},
		{
			dir:  "/foo",
			path: "/bar/ffffffffffffffffffffffffffffffff-x",
			err:  true,
		},
		{
			dir:  "/foo",
			path: "/foo/ffffffffffffffffffffffffffffffff-x/../../bar/ffffffffffffffffffffffffffffffff-x",
			err:  true,
		},
		{
			dir:  "/nix",
			path: "/nix/store",
			err:  true,
		},
		{
			dir:  "/nix/store",
			path: "/nix/store/00bgd045z0d4icpbc2yyz4gx48ak44la-net-tools-1.60_p20170221182432/bin/arp",
			want: "/nix/store/00bgd045z0d4icpbc2yyz4gx48ak44la-net-tools-1.60_p20170221182432",
			sub:  "bin/arp",
		},
	}
	for _, test := range storePathTests {
		if test.err {
			continue
		}
		tests = append(tests, parsePathTest{
			dir:  test.dir,
			path: test.path,
			want: StorePath(slashpath.Clean(test.path)),
		})
	}

	for _, test := range tests {
		got, sub, err := test.dir.ParsePath(test.path)
		if got != test.want || sub != test.sub || (err != nil) != test.err {
			errString := "<nil>"
			if test.err {
				errString = "<error>"
			}
			t.Errorf("StoreDirectory(%q).ParsePath(%q) = %q, %q, %v; want %q, %q, %s",
				test.dir, test.path, got, sub, err, test.want, test.sub, errString)
		}
	}
}
