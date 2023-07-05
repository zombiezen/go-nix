package hash

import (
	"fmt"
	"strings"

	mh "github.com/multiformats/go-multihash/core"
	"github.com/nix-community/go-nix/nix"
	"github.com/nix-community/go-nix/pkg/nixbase32"
)

//nolint:gochecknoglobals
var hashtypeToMH = map[nix.HashType]int{
	nix.MD5:    mh.MD5,
	nix.SHA1:   mh.SHA1,
	nix.SHA256: mh.SHA2_256,
	nix.SHA512: mh.SHA2_512,
}

// ParseNixBase32 returns a new Hash struct, by parsing a hashtype:nixbase32 string, or an error.
// It only supports parsing strings specifying sha1, sha256 and sha512 hashtypes,
// as Nix doesn't support other hash types.
func ParseNixBase32(s string) (*Hash, error) {
	i := strings.IndexByte(s, ':')
	if i <= 0 {
		return nil, fmt.Errorf("unable to find separator in %v", s)
	}

	hashType, err := nix.ParseHashType(s[:i])
	if err != nil {
		return nil, err
	}
	encodedDigestStr := s[i+1:]
	if len(encodedDigestStr) != nixbase32.EncodedLen(hashType.Size()) {
		return nil, fmt.Errorf("invalid length for encoded digest line %v", s)
	}
	parsed, err := nix.ParseHash(s)
	if err != nil {
		return nil, err
	}

	mhType := hashtypeToMH[hashType]
	h, err := mh.GetHasher(uint64(mhType))
	if err != nil {
		return nil, err
	}

	return &Hash{
		HashType: mhType,
		// even though the hash function is never written too, we still keep it around, for h.hash.Size() checks etc.
		hash:   h,
		digest: parsed.Bytes(nil),
	}, nil
}
