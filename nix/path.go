package nix

import (
	"fmt"
	"os"
	slashpath "path"
	"path/filepath"
	"strings"

	"github.com/nix-community/go-nix/pkg/nixbase32"
)

// ParseStorePath parses an absolute slash-separated path as a [store path]
// (i.e. an immediate child of a Nix store directory)
// and returns the directory and store object name.
//
// [store path]: https://nixos.org/manual/nix/stable/glossary.html#gloss-store-path
func ParseStorePath(path string) (StoreDirectory, ObjectName, error) {
	if !slashpath.IsAbs(path) {
		return "", "", fmt.Errorf("parse nix store path %s: not absolute", path)
	}
	dir, base := slashpath.Split(path)
	dir = slashpath.Clean(dir)
	name, err := ParseObjectName(base)
	if err != nil {
		return StoreDirectory(dir), "", fmt.Errorf("parse nix store path %s: %v", path, err)
	}
	return StoreDirectory(dir), name, nil
}

// StoreDirectory is the location of a Nix store in the local filesystem.
type StoreDirectory string

// DefaultStoreDirectory is the default Nix store directory.
const DefaultStoreDirectory StoreDirectory = "/nix/store"

// StoreDirectoryFromEnv returns the Nix store directory in use
// based on the NIX_STORE_DIR environment variable,
// falling back to [DefaultStoreDirectory] if not set.
func StoreDirectoryFromEnv() (StoreDirectory, error) {
	dir := os.Getenv("NIX_STORE_DIR")
	if dir == "" {
		return DefaultStoreDirectory, nil
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("find nix store directory: %q is not absolute", dir)
	}
	return StoreDirectory(filepath.Clean(dir)), nil
}

// Path returns the store path for the given store object name.
func (dir StoreDirectory) Path(name ObjectName) string {
	return slashpath.Join(string(dir), string(name))
}

// ParsePath verifies that a given absolute slash-separated path
// is begins with the store directory
// and names either a store object or a file inside a store object.
// On success, it returns the store object's name
// and the relative path inside the store object, if any.
func (dir StoreDirectory) ParsePath(path string) (name ObjectName, sub string, err error) {
	if !slashpath.IsAbs(string(dir)) {
		return "", "", fmt.Errorf("parse nix store path %s: directory %s not absolute", path, dir)
	}
	if !slashpath.IsAbs(path) {
		return "", "", fmt.Errorf("parse nix store path %s: not absolute", path)
	}
	cleaned := slashpath.Clean(path)
	dirPrefix := slashpath.Clean(string(dir)) + "/"
	tail, ok := cutPrefix(cleaned, dirPrefix)
	if !ok {
		return "", "", fmt.Errorf("parse nix store path %s: outside %s", path, dir)
	}
	childName, sub, _ := strings.Cut(tail, "/")
	name, err = ParseObjectName(childName)
	if err != nil {
		return "", "", fmt.Errorf("parse nix store path %s: %v", path, err)
	}
	return name, sub, nil
}

// ObjectName is the file name of a Nix [store object].
// It includes both a hash and a human-readable name,
// but no leading directory.
// For example: "s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1".
//
// [store object]: https://nixos.org/manual/nix/stable/glossary.html#gloss-store-object
type ObjectName string

const (
	objectNameHashLength = 32
	maxObjectNameLength  = objectNameHashLength + 1 + 211
)

// ParseObjectName validates a string as the file name of a Nix store object,
// returning the error encountered if any.
func ParseObjectName(name string) (ObjectName, error) {
	if len(name) < objectNameHashLength+len("-")+1 {
		return "", fmt.Errorf("parse nix store object name: %q is too short", name)
	}
	if len(name) > maxObjectNameLength {
		return "", fmt.Errorf("parse nix store object name: %q is too long", name)
	}
	for i := 0; i < len(name); i++ {
		if !isNameChar(name[i]) {
			return "", fmt.Errorf("parse nix store object name: %q contains illegal character %q", name, name[i])
		}
	}
	for i := 0; i < objectNameHashLength; i++ {
		if !nixbase32.Is(name[i]) {
			return "", fmt.Errorf("parse nix store object name: %q contains illegal base-32 character %q", name, name[i])
		}
	}
	if name[objectNameHashLength] != '-' {
		return "", fmt.Errorf("parse nix store object name: %q does not separate hash with dash", name)
	}
	return ObjectName(name), nil
}

// IsDerivation reports whether the name ends in ".drv".
func (name ObjectName) IsDerivation() bool {
	return strings.HasSuffix(string(name), ".drv")
}

// Hash returns the hash part of the name.
func (name ObjectName) Hash() string {
	if len(name) < objectNameHashLength {
		return ""
	}
	return string(name[:objectNameHashLength])
}

// Name returns the part of the name after the hash.
func (name ObjectName) Name() string {
	if len(name) <= objectNameHashLength+len("-") {
		return ""
	}
	return string(name[objectNameHashLength+len("-"):])
}

func cutPrefix(s, prefix string) (after string, found bool) {
	if !strings.HasPrefix(s, prefix) {
		return s, false
	}
	return s[len(prefix):], true
}

func isNameChar(c byte) bool {
	return 'a' <= c && c <= 'z' ||
		'A' <= c && c <= 'Z' ||
		'0' <= c && c <= '9' ||
		c == '+' || c == '-' || c == '.' || c == '_' || c == '?' || c == '='
}
