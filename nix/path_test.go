package nix

import (
	slashpath "path"
	"strings"
	"testing"
)

var storePathTests = []struct {
	path string
	dir  StoreDirectory
	name ObjectName
}{
	{
		path: "/nix/store/ffffffffffffffffffffffffffffffff-x",
		dir:  "/nix/store",
		name: "ffffffffffffffffffffffffffffffff-x",
	},
	{
		path: "/nix/store/foo/../ffffffffffffffffffffffffffffffff-x",
		dir:  "/nix/store",
		name: "ffffffffffffffffffffffffffffffff-x",
	},
	{
		path: "/nix/store/s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1",
		dir:  "/nix/store",
		name: "s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1",
	},
	{
		path: "/nix/store/s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1",
		dir:  "/nix/store",
		name: "s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1",
	},
}

func TestParseStorePath(t *testing.T) {
	for _, test := range storePathTests {
		dir, name, err := ParseStorePath(test.path)
		if err != nil || dir != test.dir || name != test.name {
			t.Errorf("ParseStorePath(%q) = %q, %q, %v; want %q, %q, <nil>", test.path, dir, name, err, test.dir, test.name)
		}
	}
	badPaths := []string{
		"",
		"foo",                                    // relative and not an object name
		"foo/ffffffffffffffffffffffffffffffff-x", // relative
		"/nix/store",                             // final path component not a name
	}
	for _, input := range badPaths {
		dir, name, err := ParseStorePath(input)
		if err == nil {
			t.Errorf("ParseStorePath(%q) = %q, %q, <nil>; want _, _, <error>", input, dir, name)
		}
	}
}

func TestStoreDirectoryPath(t *testing.T) {
	for _, test := range storePathTests {
		got := test.dir.Path(test.name)
		want := slashpath.Clean(test.path)
		if got != want {
			t.Errorf("StoreDirectory(%q).Path(%q) = %q; want %q", test.dir, test.name, got, want)
		}
	}
}

func TestStoreDirectoryParsePath(t *testing.T) {
	type parsePathTest struct {
		dir  StoreDirectory
		path string

		name ObjectName
		sub  string
		err  bool
	}
	tests := []parsePathTest{
		{
			dir:  "/nix/store",
			path: "/nix/store/s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1/bin/hello",

			name: "s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1",
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
			name: "00bgd045z0d4icpbc2yyz4gx48ak44la-net-tools-1.60_p20170221182432",
			sub:  "bin/arp",
		},
	}
	for _, test := range storePathTests {
		tests = append(tests, parsePathTest{
			dir:  test.dir,
			path: test.path,
			name: test.name,
		})
	}

	for _, test := range tests {
		name, sub, err := test.dir.ParsePath(test.path)
		if name != test.name || sub != test.sub || (err != nil) != test.err {
			errString := "<nil>"
			if test.err {
				errString = "<error>"
			}
			t.Errorf("StoreDirectory(%q).ParsePath(%q) = %q, %q, %v; want %q, %q, %s",
				test.dir, test.path, name, sub, err, test.name, test.sub, errString)
		}
	}
}

var objectNameTests = []struct {
	name         ObjectName
	hashPart     string
	namePart     string
	isDerivation bool
	valid        bool
}{
	{
		name:     "",
		hashPart: "",
		namePart: "",
		valid:    false,
	},
	{
		name:     "ffffffffffffffffffffffffffffffff-x",
		hashPart: "ffffffffffffffffffffffffffffffff",
		namePart: "x",
		valid:    true,
	},
	{
		name:     "s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1",
		hashPart: "s66mzxpvicwk07gjbjfw9izjfa797vsw",
		namePart: "hello-2.12.1",
		valid:    true,
	},
	{
		name:         "ib3sh3pcz10wsmavxvkdbayhqivbghlq-hello-2.12.1.drv",
		hashPart:     "ib3sh3pcz10wsmavxvkdbayhqivbghlq",
		namePart:     "hello-2.12.1.drv",
		isDerivation: true,
		valid:        true,
	},
	{
		name:     "00bgd045z0d4icpbc2yyz4gx48ak44la-net-tools-1.60_p20170221182432",
		hashPart: "00bgd045z0d4icpbc2yyz4gx48ak44la",
		namePart: "net-tools-1.60_p20170221182432",
		valid:    true,
	},
}

func TestObjectName(t *testing.T) {
	t.Run("Hash", func(t *testing.T) {
		for _, test := range objectNameTests {
			if got, want := test.name.Hash(), test.hashPart; got != want {
				t.Errorf("ObjectName(%q).Hash() = %q; want %q", test.name, got, want)
			}
		}
	})

	t.Run("Name", func(t *testing.T) {
		for _, test := range objectNameTests {
			if got, want := test.name.Name(), test.namePart; got != want {
				t.Errorf("ObjectName(%q).Name() = %q; want %q", test.name, got, want)
			}
		}
	})

	t.Run("IsDerivation", func(t *testing.T) {
		for _, test := range objectNameTests {
			if got, want := test.name.IsDerivation(), test.isDerivation; got != want {
				t.Errorf("ObjectName(%q).IsDerivation() = %t; want %t", test.name, got, want)
			}
		}
	})
}

func TestParseObjectName(t *testing.T) {
	for _, test := range objectNameTests {
		got, err := ParseObjectName(string(test.name))
		switch {
		case err != nil && test.valid:
			t.Errorf("ParseObjectName(%q) = %q, %v; want %q, <nil>", test.name, got, err, test.name)
		case !test.valid && (got != "" || err == nil):
			t.Errorf("ParseObjectName(%q) = %q, %v; want \"\", <error>", test.name, got, err)
		}
	}

	badNames := []string{
		"ffffffffffffffffffffffffffffffff",
		"ffffffffffffffffffffffffffffffff-",
		"ffffffffffffffffffffffffffffffff_x",
		"ffffffffffffffffffffffffffffffff-" + strings.Repeat("x", 212),
		"ffffffffffffffffffffffffffffffff-foo@bar",
		"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee-x",
		"00bgd045z0d4icpbc2yy-net-tools-1.60_p20170221182432",
		"00bgd045z0d4icpbc2yyz4gx48aku4la-net-tools-1.60_p20170221182432",
	}
	for _, name := range badNames {
		got, err := ParseObjectName(string(name))
		if got != "" || err == nil {
			t.Errorf("ParseObjectName(%q) = %q, %v; want \"\", <error>", name, got, err)
		}
	}
}
