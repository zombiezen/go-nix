package nix

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"

	"github.com/nix-community/go-nix/nixbase32"
)

// base64Encoding is the Nix base64 alphabet.
var base64Encoding = base64.StdEncoding

// HashType is an enumeration of algorithms supported by [Hash].
type HashType int8

// Hash algorithms.
// Applications must not depend on the exact numeric values.
const (
	MD5 HashType = 1 + iota
	SHA1
	SHA256
	SHA512
)

// ParseHashType matches a string to its hash type,
// returning an error if the string does not name a hash type.
func ParseHashType(s string) (HashType, error) {
	allTypes := [...]HashType{MD5, SHA1, SHA256, SHA512}
	for _, typ := range allTypes {
		if s == typ.String() {
			return typ, nil
		}
	}
	return 0, fmt.Errorf("%q is not a hash type", s)
}

// IsValid reports whether typ is one of the known hash algorithms.
func (typ HashType) IsValid() bool {
	return typ == MD5 || typ == SHA1 || typ == SHA256 || typ == SHA512
}

// Size returns the size of a hash produced by this type in bytes.
func (typ HashType) Size() int {
	switch typ {
	case 0:
		return 0
	case MD5:
		return md5.Size
	case SHA1:
		return sha1.Size
	case SHA256:
		return sha256.Size
	case SHA512:
		return sha512.Size
	default:
		panic("invalid hash type")
	}
}

// String returns the name of the hash algorithm.
func (typ HashType) String() string {
	switch typ {
	case MD5:
		return "md5"
	case SHA1:
		return "sha1"
	case SHA256:
		return "sha256"
	case SHA512:
		return "sha512"
	default:
		return fmt.Sprintf("HashType(%d)", int(typ))
	}
}

// A Hash is an output of a hash algorithm.
// The zero value is an empty hash with no type.
type Hash struct {
	_    [0]func() // Prevent comparisons for future-proofing.
	typ  HashType
	hash [sha512.Size]byte
}

// NewHash returns a new hash with the given type and raw bytes.
// NewHash panics if the type is invalid
// or the length of the raw bytes do not match the type's length.
func NewHash(typ HashType, bits []byte) Hash {
	if !typ.IsValid() {
		panic("invalid hash type")
	}
	if len(bits) != typ.Size() {
		panic("hash size does not match hash type")
	}
	h := Hash{typ: typ}
	copy(h.hash[:], bits)
	return h
}

// ParseHash parses a hash
// in the format "<type>:<base16|base32|base64>" or "<type>-<base64>"
// (a [Subresource Integrity hash expression]).
// It is a wrapper around [Hash.UnmarshalText].
//
// [Subresource Integrity hash expression]: https://www.w3.org/TR/SRI/#the-integrity-attribute
func ParseHash(s string) (Hash, error) {
	var h Hash
	if err := h.UnmarshalText([]byte(s)); err != nil {
		return Hash{}, err
	}
	return h, nil
}

// Type returns the hash's algorithm.
// It returns zero for a zero Hash.
func (h Hash) Type() HashType {
	return h.typ
}

// IsZero reports whether the hash is the zero hash.
func (h Hash) IsZero() bool {
	return h.typ == 0
}

// Equal reports whether h == h2.
func (h Hash) Equal(h2 Hash) bool {
	return h.typ == h2.typ && h.hash == h2.hash
}

// Bytes appends the raw bytes of the hash to dst
// and returns the resulting slice.
func (h Hash) Bytes(dst []byte) []byte {
	return append(dst, h.hash[:h.typ.Size()]...)
}

// String returns the result of [Hash.SRI]
// or "<nil>" if the hash is the zero Hash.
func (h Hash) String() string {
	if h.typ == 0 {
		return "<nil>"
	}
	return h.SRI()
}

// Base16 encodes the hash with base16 (i.e. hex)
// prefixed by the hash type separated by a colon.
func (h Hash) Base16() string {
	return string(h.encode(true, hex.EncodedLen, base16Encode))
}

// RawBase16 encodes the hash with base16 (i.e. hex).
func (h Hash) RawBase16() string {
	return string(h.encode(false, hex.EncodedLen, base16Encode))
}

func base16Encode(dst, src []byte) {
	hex.Encode(dst, src)
}

// Base32 encodes the hash with base32
// prefixed by the hash type separated by a colon.
func (h Hash) Base32() string {
	return string(h.encode(true, nixbase32.EncodedLen, nixbase32.Encode))
}

// RawBase32 encodes the hash with base32.
func (h Hash) RawBase32() string {
	return string(h.encode(false, nixbase32.EncodedLen, nixbase32.Encode))
}

// Base64 encodes the hash with base64
// prefixed by the hash type separated by a colon.
func (h Hash) Base64() string {
	return string(h.encode(true, base64Encoding.EncodedLen, base64Encoding.Encode))
}

// RawBase64 encodes the hash with base64.
func (h Hash) RawBase64() string {
	return string(h.encode(false, base64Encoding.EncodedLen, base64Encoding.Encode))
}

// SRI returns the hash in the format of a [Subresource Integrity hash expression]
// (e.g. "sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=").
//
// [Subresource Integrity hash expression]: https://www.w3.org/TR/SRI/#the-integrity-attribute
func (h Hash) SRI() string {
	b, _ := h.MarshalText()
	return string(b)
}

// MarshalText formats the hash as a [Subresource Integrity hash expression]
// (e.g. "sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=").
// It returns an error if h is the zero Hash.
//
// [Subresource Integrity hash expression]: https://www.w3.org/TR/SRI/#the-integrity-attribute
func (h Hash) MarshalText() ([]byte, error) {
	if h.typ == 0 {
		return nil, fmt.Errorf("cannot marshal zero hash")
	}
	buf := h.encode(true, base64Encoding.EncodedLen, base64Encoding.Encode)
	buf[bytes.IndexByte(buf, ':')] = '-'
	return buf, nil
}

// UnmarshalText parses a hash
// in the format "<type>:<base16|base32|base64>" or "<type>-<base64>"
// (a [Subresource Integrity hash expression]).
//
// [Subresource Integrity hash expression]: https://www.w3.org/TR/SRI/#the-integrity-attribute
func (h *Hash) UnmarshalText(s []byte) error {
	sep := [1]byte{':'}
	prefix, rest, hasPrefix := bytes.Cut(s, sep[:])
	isSRI := false
	if !hasPrefix {
		sep[0] = '-'
		prefix, rest, isSRI = bytes.Cut(s, sep[:])
		if !isSRI {
			return fmt.Errorf("parse hash %q: missing prefix", s)
		}
	}
	var err error
	h.typ, err = ParseHashType(string(prefix))
	if err != nil {
		return fmt.Errorf("parse hash %q: %v", s, err)
	}
	switch {
	case isSRI && len(rest) != base64Encoding.EncodedLen(h.typ.Size()):
		return fmt.Errorf("parse hash %q: wrong length for SRI of type %v", s, h.typ)
	case len(rest) == hex.EncodedLen(h.typ.Size()):
		if _, err := hex.Decode(h.hash[:], rest); err != nil {
			return fmt.Errorf("parse hash %q: %v", s, err)
		}
	case len(rest) == nixbase32.EncodedLen(h.typ.Size()):
		if _, err := nixbase32.Decode(h.hash[:], rest); err != nil {
			return fmt.Errorf("parse hash %q: %v", s, err)
		}
	case len(rest) == base64Encoding.EncodedLen(h.typ.Size()):
		if _, err := base64Encoding.Decode(h.hash[:], rest); err != nil {
			return fmt.Errorf("parse hash %q: %v", s, err)
		}
	default:
		return fmt.Errorf("parse hash %q: wrong length for hash of type %v", s, h.typ)
	}
	return nil
}

func (h Hash) encode(includeType bool, encodedLen func(int) int, encode func(dst, src []byte)) []byte {
	if h.typ == 0 {
		return nil
	}
	hashLen := h.typ.Size()
	n := encodedLen(hashLen)
	if includeType {
		n += len(h.typ.String()) + 1
	}

	buf := make([]byte, 0, n)
	if includeType {
		buf = append(buf, h.typ.String()...)
		buf = append(buf, ':')
	}
	encode(buf[len(buf):n], h.hash[:hashLen])
	return buf[:n]
}

// Hasher is a common interface for hash algorithms to produce [Hash] values
// from byte streams.
// Hasher implements [hash.Hash].
type Hasher struct {
	typ  HashType
	hash hash.Hash
}

// NewHasher returns a new Hasher object for the given algorithm.
// NewHasher panics if the hash type is invalid.
func NewHasher(typ HashType) *Hasher {
	h := &Hasher{typ: typ}
	switch typ {
	case MD5:
		h.hash = md5.New()
	case SHA1:
		h.hash = sha1.New()
	case SHA256:
		h.hash = sha256.New()
	case SHA512:
		h.hash = sha512.New()
	default:
		panic("invalid hash type")
	}
	return h
}

// Type returns the type passed into [NewHasher].
func (h *Hasher) Type() HashType {
	return h.typ
}

// Write adds more data to the running hash.
// It never returns an error.
func (h *Hasher) Write(p []byte) (n int, err error) {
	return h.hash.Write(p)
}

// Sum appends the current hash to b and returns the resulting slice.
// It does not change the underlying hash state.
func (h *Hasher) Sum(b []byte) []byte {
	return h.hash.Sum(b)
}

// SumHash returns the current [Hash] value.
// It does not change the underlying hash state.
func (h *Hasher) SumHash() Hash {
	h2 := Hash{typ: h.typ}
	h.hash.Sum(h2.hash[:0])
	return h2
}

// Reset resets the hasher to its initial state.
func (h *Hasher) Reset() {
	h.hash.Reset()
}

// Size returns the number of bytes [Hasher.Sum] will return.
func (h *Hasher) Size() int {
	return h.hash.Size()
}

// BlockSize returns the hash's underlying block size.
func (h *Hasher) BlockSize() int {
	return h.hash.BlockSize()
}

// CompressHash compresses the src byte slice (usually a hash digest)
// into the given dst byte slice by cyclically XORing bytes together.
// If len(dst) >= len(src), dst[:len(src)] will be a copy of src
// and dst[len(src):] will not be modified.
func CompressHash(dst, src []byte) {
	n := copy(dst, src)
	if n == len(src) {
		return
	}
	for i := n; i < len(src); i++ {
		dst[i%len(dst)] ^= src[i]
	}
}
