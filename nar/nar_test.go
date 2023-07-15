package nar

import "testing"

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
