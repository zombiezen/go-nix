// Package nixpath parses and renders Nix store paths.
//
// Deprecated: Use the functions in the [nix] package instead.
package nixpath

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/nix-community/go-nix/nix"
	"github.com/nix-community/go-nix/pkg/nixbase32"
)

const (
	StoreDir     = "/nix/store"
	PathHashSize = 20
)

//nolint:gochecknoglobals
var (
	NameRe = regexp.MustCompile(`[a-zA-Z0-9+\-_?=][.a-zA-Z0-9+\-_?=]*`)

	// Length of the hash portion of the store path in base32.
	encodedPathHashSize = nixbase32.EncodedLen(PathHashSize)

	// Offset in path string to name.
	nameOffset = len(StoreDir) + 1 + encodedPathHashSize + 1
	// Offset in path string to hash.
	hashOffset = len(StoreDir) + 1
)

// NixPath represents a bare nix store path, without any paths underneath `/nix/store/…-…`.
//
// Deprecated: Use [nix.ObjectName] instead.
type NixPath struct {
	Name   string
	Digest []byte
}

func (n *NixPath) String() string {
	return Absolute(nixbase32.EncodeToString(n.Digest) + "-" + n.Name)
}

func (n *NixPath) Validate() error {
	return Validate(n.String())
}

// FromString parses a path string into a nix path,
// verifying it's syntactically valid
// It returns an error if it fails to parse.
//
// Deprecated: Use [nix.ParseStorePath].
func FromString(s string) (*NixPath, error) {
	dir, name, err := nix.ParseStorePath(s)
	if err != nil {
		return nil, err
	}
	if dir != nix.DefaultStoreDirectory {
		return nil, fmt.Errorf("unable to parse path: mismatching store path prefix for path %v", s)
	}

	digest, err := nixbase32.DecodeString(name.Hash())
	if err != nil {
		return nil, fmt.Errorf("unable to decode hash: %v", err)
	}

	return &NixPath{
		Name:   name.Name(),
		Digest: digest,
	}, nil
}

// Absolute prefixes a nixpath name with StoreDir and a '/', and cleans the path.
// It does not prevent from leaving StoreDir, so check if it still starts with StoreDir
// if you accept untrusted input.
// This should be used when assembling store paths in hashing contexts.
// Even if this code is running on windows, we want to use forward
// slashes to construct them.
//
// Deprecated: Use [nix.StoreDirectory.StorePath].
func Absolute(name string) string {
	return path.Join(StoreDir, name)
}

// Validate validates a path string, verifying it's syntactically valid.
//
// Deprecated: Use one of [nix.ParseStorePath] or [nix.ParseObjectName]
// depending on your needs.
func Validate(s string) error {
	name, ok := cutPrefix(s, StoreDir+"/")
	if !ok {
		return fmt.Errorf("unable to parse path: mismatching store path prefix for path %v", s)
	}
	if _, err := nix.ParseObjectName(name); err != nil {
		return fmt.Errorf("unable to parse path: %v", err)
	}
	return nil
}

func cutPrefix(s, prefix string) (after string, found bool) {
	if !strings.HasPrefix(s, prefix) {
		return s, false
	}
	return s[len(prefix):], true
}
