package nix

import (
	"fmt"
	"os"
	slashpath "path"
	"path/filepath"
	"strings"

	"zombiezen.com/go/nix/nixbase32"
)

// StoreDirectory is the absolute path of a Nix store in the local filesystem.
type StoreDirectory string

// DefaultStoreDirectory is the default Nix store directory.
const DefaultStoreDirectory StoreDirectory = "/nix/store"

// CleanStoreDirectory cleans an absolute slash-separated path as a [StoreDirectory].
// It returns an error if the path is not absolute.
func CleanStoreDirectory(path string) (StoreDirectory, error) {
	if !slashpath.IsAbs(path) {
		return "", fmt.Errorf("store directory %q is not absolute", path)
	}
	return StoreDirectory(slashpath.Clean(path)), nil
}

// StoreDirectoryFromEnvironment returns the Nix store directory in use
// based on the NIX_STORE_DIR environment variable,
// falling back to [DefaultStoreDirectory] if not set.
func StoreDirectoryFromEnvironment() (StoreDirectory, error) {
	dir := os.Getenv("NIX_STORE_DIR")
	if dir == "" {
		return DefaultStoreDirectory, nil
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("find nix store directory: %q is not absolute", dir)
	}
	return StoreDirectory(filepath.Clean(dir)), nil
}

// Object returns the store path for the given store object name.
func (dir StoreDirectory) Object(name string) (StorePath, error) {
	joined := dir.Join(name)
	if name == "" || name == "." || name == ".." || strings.Contains(name, "/") {
		return "", fmt.Errorf("parse nix store path %s: invalid object name %q", joined, name)
	}
	storePath, err := ParseStorePath(joined)
	if err != nil {
		return "", err
	}
	return storePath, nil
}

// Join joins any number of path elements to the store directory
// separated by slashes.
func (dir StoreDirectory) Join(elem ...string) string {
	return slashpath.Join(append([]string{string(dir)}, elem...)...)
}

// ParsePath verifies that a given absolute slash-separated path
// begins with the store directory
// and names either a store object or a file inside a store object.
// On success, it returns the store object's name
// and the relative path inside the store object, if any.
func (dir StoreDirectory) ParsePath(path string) (storePath StorePath, sub string, err error) {
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
	storePath, err = ParseStorePath(cleaned[:len(dirPrefix)+len(childName)])
	if err != nil {
		return "", "", err
	}
	return storePath, sub, nil
}

// StorePath is a Nix [store path]:
// the absolute path of a Nix [store object] in the filesystem.
// For example: "/nix/store/s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1".
//
// [store object]: https://nixos.org/manual/nix/stable/glossary.html#gloss-store-object
// [store path]: https://nixos.org/manual/nix/stable/glossary.html#gloss-store-path
type StorePath string

const (
	objectNameDigestLength = 32
	maxObjectNameLength    = objectNameDigestLength + 1 + 211
)

// ParseStorePath parses an absolute slash-separated path as a [store path]
// (i.e. an immediate child of a Nix store directory).
//
// [store path]: https://nixos.org/manual/nix/stable/glossary.html#gloss-store-path
func ParseStorePath(path string) (StorePath, error) {
	if !slashpath.IsAbs(path) {
		return "", fmt.Errorf("parse nix store path %s: not absolute", path)
	}
	cleaned := slashpath.Clean(path)
	_, base := slashpath.Split(cleaned)
	if len(base) < objectNameDigestLength+len("-")+1 {
		return "", fmt.Errorf("parse nix store path %s: %q is too short", path, base)
	}
	if len(base) > maxObjectNameLength {
		return "", fmt.Errorf("parse nix store path %s: %q is too long", path, base)
	}
	for i := 0; i < len(base); i++ {
		if !isNameChar(base[i]) {
			return "", fmt.Errorf("parse nix store path %s: %q contains illegal character %q", path, base, base[i])
		}
	}
	if err := nixbase32.ValidateString(base[:objectNameDigestLength]); err != nil {
		return "", fmt.Errorf("parse nix store path %s: %v", path, err)
	}
	if base[objectNameDigestLength] != '-' {
		return "", fmt.Errorf("parse nix store path %s: digest not separated by dash", path)
	}
	return StorePath(cleaned), nil
}

// Dir returns the path's directory.
func (path StorePath) Dir() StoreDirectory {
	if path == "" {
		return ""
	}
	return StoreDirectory(slashpath.Dir(string(path)))
}

// Base returns the last element of the path.
func (path StorePath) Base() string {
	if path == "" {
		return ""
	}
	return slashpath.Base(string(path))
}

// IsDerivation reports whether the name ends in ".drv".
func (path StorePath) IsDerivation() bool {
	return strings.HasSuffix(path.Base(), ".drv")
}

// Digest returns the digest part of the name.
func (path StorePath) Digest() string {
	base := path.Base()
	if len(base) < objectNameDigestLength {
		return ""
	}
	return string(base[:objectNameDigestLength])
}

// Name returns the part of the name after the digest.
func (path StorePath) Name() string {
	base := path.Base()
	if len(base) <= objectNameDigestLength+len("-") {
		return ""
	}
	return string(base[objectNameDigestLength+len("-"):])
}

// MarshalText returns a byte slice of the path
// or an error if it's empty.
func (path StorePath) MarshalText() ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("marshal nix store path: empty")
	}
	return []byte(path), nil
}

// UnmarshalText validates and cleans the path in the same way as [ParseStorePath]
// and stores it into *path.
func (path *StorePath) UnmarshalText(data []byte) error {
	var err error
	*path, err = ParseStorePath(string(data))
	if err != nil {
		return err
	}
	return nil
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
