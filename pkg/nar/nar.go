/*
Package nar implements access to Nix Archive (NAR) files.

Nix Archive is a file format for storing a directory or a single file
in a binary reproducible format. This is the format that is being used to
pack and distribute Nix build results. It doesn't store any timestamps or
similar fields available in conventional filesystems. .nar files can be read
and written in a streaming manner.
*/
package nar

import (
	"fmt"
	"io/fs"
	slashpath "path"
	"strings"
	"time"
	"unicode/utf8"
)

// A Header represents a single header in a NAR archive.
// Some fields may not be populated.
type Header struct {
	// Path is a UTF-8 encoded, unrooted, slash-separated sequence of path elements,
	// like "x/y/z".
	// Path will not contain elements that are "." or ".." or the empty string,
	// except for the special case that the root of the archive is the empty string.
	Path string
	// Mode is the type of the file system object.
	// During writing, the permission bits are largely ignored
	// except the executable bits for a regular file:
	// if any are non-zero, then the file will be marked as executable.
	Mode fs.FileMode
	// Size is the size of a regular file in bytes.
	Size int64
	// LinkTarget is the target of a symlink.
	LinkTarget string
}

// Validate does some consistency checking of the header structure, such as
// checking for valid paths and inconsistent fields, and returns an error if it
// fails validation.
func (h *Header) Validate() error {
	// Path needs to start with a /, and must not contain null bytes
	// as we might get passed windows paths, ToSlash them first.
	if err := validatePath(h.Path); err != nil {
		return err
	}

	if strings.ContainsAny(h.Path, "\u0000") {
		return fmt.Errorf("path may not contain null bytes")
	}

	// Regular files and directories may not have LinkTarget set.
	if h.Mode.Type() != fs.ModeSymlink && h.LinkTarget != "" {
		return fmt.Errorf("mode is %v, but Linkname is not empty", h.Mode)
	}

	// Directories and Symlinks may not have Size and Executable set.
	if h.Mode.Type() != 0 && h.Size != 0 {
		return fmt.Errorf("mode is %v, but Size is not 0", h.Mode)
	}

	// Symlinks need to specify a target.
	if h.Mode.Type() == fs.ModeSymlink && h.LinkTarget == "" {
		return fmt.Errorf("type is symlink, but Linkname is empty")
	}

	return nil
}

// FileInfo returns an fs.FileInfo for the Header.
func (h *Header) FileInfo() fs.FileInfo {
	return headerFileInfo{h}
}

type headerFileInfo struct {
	h *Header
}

func (fi headerFileInfo) Mode() fs.FileMode  { return fi.h.Mode }
func (fi headerFileInfo) Size() int64        { return fi.h.Size }
func (fi headerFileInfo) IsDir() bool        { return fi.h.Mode.IsDir() }
func (fi headerFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (fi headerFileInfo) Sys() any           { return fi.h }

func (fi headerFileInfo) Name() string {
	if fi.h.Path == "" {
		return ""
	}
	return slashpath.Base(fi.h.Path)
}

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
	if !utf8.ValidString(name) {
		return fmt.Errorf("filename is not UTF-8")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("filename %q is reserved", name)
	}
	if i := strings.IndexAny(name, "\x00/"); i != -1 {
		return fmt.Errorf("%q not allowed in filename", name[i])
	}
	return nil
}
