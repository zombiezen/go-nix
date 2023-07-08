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
	}
	for _, test := range tests {
		pop, newDirs, err := treeDelta(test.oldPath, test.oldIsDir, test.newPath)
	}
}
