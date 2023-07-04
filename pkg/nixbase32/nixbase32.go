/*
Package nixbase32 implements the slightly odd "base32" encoding that's used
in Nix.

Nix uses a custom alphabet. Contrary to other implementations (RFC4648),
encoding to "nix base32" also reads in characters in reverse order (and
doesn't use any padding), which makes adopting encoding/base32 hard.
This package provides some of the functions defined in
encoding/base32.Encoding.
*/
package nixbase32

import (
	"fmt"
	"strings"
)

// alphabet contains the list of valid characters for the Nix base32 alphabet.
const alphabet = "0123456789abcdfghijklmnpqrsvwxyz"

// DecodeString returns the bytes represented by the nixbase32 string s or
// returns an error.
func DecodeString(s string) ([]byte, error) {
	dst := make([]byte, DecodedLen(len(s)))

	for n := 0; n < len(s); n++ {
		c := s[len(s)-n-1]

		digit := strings.IndexByte(alphabet, c)
		if digit == -1 {
			return nil, fmt.Errorf("decode base32: character %q not in alphabet", c)
		}

		b := uint64(n * 5)
		i := int(b / 8)
		j := int(b % 8)

		// OR the main pattern
		dst[i] |= byte(digit) << j

		// calculate the "carry pattern"
		carry := byte(digit) >> (8 - j)

		if i+1 < len(dst) {
			dst[i+1] |= carry
		} else if carry != 0 {
			// but have a nonzero carry, the encoding is invalid.
			return nil, fmt.Errorf("decode base32: non-zero padding")
		}
	}

	return dst, nil
}

// EncodedLen returns the length in bytes of the base32 encoding of an input
// buffer of length n.
func EncodedLen(n int) int {
	if n == 0 {
		return 0
	}

	return (n*8-1)/5 + 1
}

// DecodedLen returns the length in bytes of the decoded data
// corresponding to n bytes of base32-encoded data.
// If we have bits that don't fit into here, they are padding and must
// be 0.
func DecodedLen(n int) int {
	return (n * 5) / 8
}

// EncodeToString returns the nixbase32 encoding of src.
func EncodeToString(src []byte) string {
	l := EncodedLen(len(src))

	var dst strings.Builder

	dst.Grow(l)

	for n := l - 1; n >= 0; n-- {
		b := uint(n * 5)
		i := b / 8
		j := b % 8

		c := src[i] >> j

		if i+1 < uint(len(src)) {
			c |= src[i+1] << (8 - j)
		}

		dst.WriteByte(alphabet[c&0x1f])
	}

	return dst.String()
}

// Is reports whether the given byte is part of the nixbase32 alphabet.
func Is(c byte) bool {
	return '0' <= c && c <= '9' ||
		'a' <= c && c <= 'z' && c != 'e' && c != 'o' && c != 'u' && c != 't'
}
