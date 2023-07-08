// Package nar provides types for reading and writing Nix Archive (NAR) files.
//
// The NAR format is described in Figure 5.2 on page 93 of
// [The purely functional software deployment model] by Eelco Dolstra.
//
// [The purely functional software deployment model]: https://edolstra.github.io/pubs/phd-thesis.pdf
package nar

import (
	"fmt"
	"strings"
)

// Tokens
const (
	magic = "nix-archive-1"

	typeRegular   = "regular"
	typeDirectory = "directory"
	typeSymlink   = "symlink"

	typeToken = "type"

	executableToken = "executable"
	contentsToken   = "contents"
	targetToken     = "target"

	entryToken = "entry"
	nameToken  = "name"
	nodeToken  = "node"
)

const (
	entryNameMaxLen     = 255
	symlinkTargetMaxLen = 4095
)

const stringAlign = 8

// padStringSize returns the smallest integer >= n
// that is evenly divisible by [stringAlign].
func padStringSize(n int) int {
	return (n + stringAlign - 1) &^ (stringAlign - 1)
}

// stringPaddingLength returns the difference between
// the result of [padStringSize] of n
// and the n.
func stringPaddingLength(n int) int {
	return (^n + 1) & (stringAlign - 1)
}

func validateFilename(name string) error {
	if name == "" {
		return fmt.Errorf("empty filename")
	}
	if len(name) > entryNameMaxLen {
		return fmt.Errorf("filename longer than %d characters", entryNameMaxLen)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("filename %q is reserved", name)
	}
	if i := strings.IndexAny(name, "\x00/"); i != -1 {
		return fmt.Errorf("%q not allowed in filename", name[i])
	}
	return nil
}
