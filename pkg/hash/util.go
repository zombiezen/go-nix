package hash

import "github.com/nix-community/go-nix/nix"

// CompressHash takes an arbitrary long sequence of bytes (usually a hash digest),
// and returns a sequence of bytes of length newSize.
// It's calculated by rotating through the bytes in the output buffer (zero-initialized),
// and XOR'ing with each byte in the passed input
// It consumes 1 byte at a time, and XOR's it with the current value in the output buffer.
//
// Deprecated: Use [nix.CompressHash].
func CompressHash(input []byte, outputSize int) []byte {
	buf := make([]byte, outputSize)
	nix.CompressHash(buf, input)
	return buf
}
