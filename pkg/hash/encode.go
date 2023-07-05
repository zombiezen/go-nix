package hash

import (
	"fmt"

	"github.com/multiformats/go-multihash"
	mh "github.com/multiformats/go-multihash/core"
	"github.com/nix-community/go-nix/nix"
)

//nolint:gochecknoglobals
var mhTypeToHashType = map[int]nix.HashType{
	mh.MD5:      nix.MD5,
	mh.SHA1:     nix.SHA1,
	mh.SHA2_256: nix.SHA256,
	mh.SHA2_512: nix.SHA512,
}

// Multihash returns the digest, in multihash format.
func (h *Hash) Multihash() []byte {
	d, _ := multihash.Encode(h.Digest(), uint64(h.HashType))
	// "The error return is legacy; it is always nil."
	return d
}

// NixString returns the string representation of a given hash, as used by Nix.
// It'll panic if another hash type is used that doesn't have
// a Nix representation.
// This is the hash type, a colon, and then the nixbase32-encoded digest
// If the hash is inconsistent (digest size doesn't match hash type, an empty
// string is returned).
func (h *Hash) NixString() string {
	digest := h.Digest()

	// This can only occur if the struct is filled manually
	if h.hash.Size() != len(digest) {
		panic("invalid digest length")
	}

	typ := mhTypeToHashType[h.HashType]
	if !typ.IsValid() {
		panic(fmt.Sprintf("unable to encode %v to nix string", h.HashType))
	}

	return nix.NewHash(typ, digest).Base32()
}

func (h *Hash) SRIString() string {
	digest := h.Digest()

	// This can only occur if the struct is filled manually
	if h.hash.Size() != len(digest) {
		panic("invalid digest length")
	}

	typ := mhTypeToHashType[h.HashType]
	if !typ.IsValid() {
		panic(fmt.Sprintf("unable to encode %v to nix string", h.HashType))
	}

	return nix.NewHash(typ, digest).SRI()
}
