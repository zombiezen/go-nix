package nix

import (
	"fmt"
	"strings"
)

type contentAddressMethod int8

const (
	textIngestionMethod contentAddressMethod = 1 + iota
	flatFileIngestionMethod
	recursiveFileIngestionMethod
)

const (
	caTextPrefix  = "text"
	caFixedPrefix = "fixed"

	caFixedRecursiveFlag = "r:"
)

// A ContentAddress is a content-addressability assertion.
type ContentAddress struct {
	_      [0]func() // Prevent comparisons for future-proofing.
	method contentAddressMethod
	hash   Hash
}

// TextContentAddress returns a content address for
// a "text" filesystem object
// with the given hash.
func TextContentAddress(h Hash) ContentAddress {
	return ContentAddress{
		method: textIngestionMethod,
		hash:   h,
	}
}

// FlatFileContentAddress returns a content address for
// a flat, fixed-output derivation
// with the given hash.
func FlatFileContentAddress(h Hash) ContentAddress {
	return ContentAddress{
		method: flatFileIngestionMethod,
		hash:   h,
	}
}

// RecursiveFileContentAddress returns a content address for
// a recursive (NAR), fixed-output derivation
// with the given hash.
func RecursiveFileContentAddress(h Hash) ContentAddress {
	return ContentAddress{
		method: recursiveFileIngestionMethod,
		hash:   h,
	}
}

// ParseContentAddress parses a content address in the form of
// "text:<ht>:<sha256 hash of file contents>" or
// "fixed<:r?>:<ht>:<h>".
func ParseContentAddress(s string) (ContentAddress, error) {
	prefix, rest, ok := strings.Cut(s, ":")
	if !ok {
		return ContentAddress{}, fmt.Errorf("parse nix content address %q: missing \"text:\" or \"fixed:\" prefix", s)
	}
	var method contentAddressMethod
	switch prefix {
	case caTextPrefix:
		method = textIngestionMethod
	case caFixedPrefix:
		var isRecursive bool
		rest, isRecursive = cutPrefix(rest, caFixedRecursiveFlag)
		if isRecursive {
			method = recursiveFileIngestionMethod
		} else {
			method = flatFileIngestionMethod
		}
	default:
		return ContentAddress{}, fmt.Errorf("parse nix content address %q: invalid prefix %q", s, prefix)
	}
	if !strings.Contains(rest, ":") {
		return ContentAddress{}, fmt.Errorf("parse nix content address %q: hash must be in form \"<algo>:<hash>\"", s)
	}
	h, err := ParseHash(rest)
	if err != nil {
		return ContentAddress{}, fmt.Errorf("parse nix content address %q: %v", s, rest)
	}
	return ContentAddress{method: method, hash: h}, nil
}

// String formats the content address as either
// "text:<ht>:<sha256 hash of file contents>" or
// "fixed<:r?>:<ht>:<h>".
// It returns the empty string if ca is the zero value.
func (ca ContentAddress) String() string {
	switch ca.method {
	case textIngestionMethod:
		return caTextPrefix + ":" + ca.hash.Base32()
	case flatFileIngestionMethod:
		return caFixedPrefix + ":" + ca.hash.Base32()
	case recursiveFileIngestionMethod:
		return caFixedPrefix + ":" + caFixedRecursiveFlag + ca.hash.Base32()
	default:
		return ""
	}
}

// IsZero reports whether the content address is the zero value.
func (ca ContentAddress) IsZero() bool {
	return ca.method == 0
}

// IsText reports whether the content address is for a "text" filesystem object.
func (ca ContentAddress) IsText() bool {
	return ca.method == textIngestionMethod
}

// IsFixed reports whether the content address is for a fixed-output derivation.
func (ca ContentAddress) IsFixed() bool {
	return ca.method == flatFileIngestionMethod || ca.method == recursiveFileIngestionMethod
}

// IsRecursiveFile reports whether the content address is
// for a fixed-output derivation with recursive (NAR) hashing.
func (ca ContentAddress) IsRecursiveFile() bool {
	return ca.method == recursiveFileIngestionMethod
}

// Hash returns the hash part of the content address.
func (ca ContentAddress) Hash() Hash {
	return ca.hash
}

// Equal reports whether ca == ca2.
func (ca ContentAddress) Equal(ca2 ContentAddress) bool {
	return ca.method == ca2.method && ca.hash.Equal(ca2.hash)
}

// MarshalText formats the content address in the same way as [ContentAddress.String].
// It returns an error if ca is the zero value.
func (ca ContentAddress) MarshalText() ([]byte, error) {
	s := ca.String()
	if s == "" {
		return nil, fmt.Errorf("marshal nix content address: invalid content address")
	}
	return []byte(s), nil
}

// UnmarshalText parses a string in the same way as [ParseContentAddress].
func (ca *ContentAddress) UnmarshalText(data []byte) error {
	newCA, err := ParseContentAddress(string(data))
	if err != nil {
		return err
	}
	*ca = newCA
	return nil
}
