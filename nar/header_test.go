package nar

import (
	"io/fs"
	"testing"
)

func TestHeaderValidate(t *testing.T) {
	headerRegular := &Header{
		Path: "foo/bar",
	}

	t.Run("valid", func(t *testing.T) {
		vHeader := *headerRegular
		if err := vHeader.Validate(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		invHeader := *headerRegular
		invHeader.Path = "/foo/bar"
		if err := invHeader.Validate(); err == nil {
			t.Error("Validate did not return an error")
		}

		invHeader.Path = "foo/bar\000/"
		if err := invHeader.Validate(); err == nil {
			t.Error("Validate did not return an error")
		}
	})

	t.Run("LinkTarget set on regulars or directories", func(t *testing.T) {
		invHeader := *headerRegular
		invHeader.LinkTarget = "foo"

		if err := invHeader.Validate(); err == nil {
			t.Error("Validate did not return an error")
		}

		invHeader.Mode = fs.ModeDir
		if err := invHeader.Validate(); err == nil {
			t.Error("Validate did not return an error")
		}
	})

	t.Run("Size set on directories or symlinks", func(t *testing.T) {
		invHeader := *headerRegular
		invHeader.Mode = fs.ModeDir
		invHeader.Size = 1
		if err := invHeader.Validate(); err == nil {
			t.Error("Validate did not return an error")
		}

		invHeader = *headerRegular
		invHeader.Mode = fs.ModeSymlink
		invHeader.Size = 1
		if err := invHeader.Validate(); err == nil {
			t.Error("Validate did not return an error")
		}
	})

	t.Run("No LinkTarget set on symlinks", func(t *testing.T) {
		invHeader := *headerRegular
		invHeader.Mode = fs.ModeSymlink
		if err := invHeader.Validate(); err == nil {
			t.Error("Validate did not return an error")
		}
	})
}
