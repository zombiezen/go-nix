package narinfo

import (
	"io"

	"github.com/nix-community/go-nix/nix"
	"github.com/nix-community/go-nix/pkg/hash"
	"github.com/nix-community/go-nix/pkg/narinfo/signature"
)

// Parse reads a .narinfo file content
// and returns a NarInfo struct with the parsed data.
//
// Deprecated: Use [nix.NARInfo.UnmarshalText].
func Parse(r io.Reader) (*NarInfo, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var info nix.NARInfo
	if err := info.UnmarshalText(data); err != nil {
		return nil, err
	}
	n := &NarInfo{
		StorePath:   info.StorePath,
		URL:         info.URL,
		Compression: string(info.Compression),
		FileHash:    toOldHash(info.FileHash),
		FileSize:    uint64(info.FileSize),
		NarHash:     toOldHash(info.NARHash),
		NarSize:     uint64(info.NARSize),
		Deriver:     string(info.Deriver),
		System:      info.System,
		CA:          info.CA.String(),
	}
	for _, ref := range info.References {
		n.References = append(n.References, string(ref))
	}
	for _, sig := range info.Sig {
		sig2, err := signature.ParseSignature(sig.String())
		if err != nil {
			return nil, err
		}
		n.Signatures = append(n.Signatures, sig2)
	}
	return n, nil
}

func toOldHash(h nix.Hash) *hash.Hash {
	if h.IsZero() {
		return nil
	}
	h2, err := hash.ParseNixBase32(h.Base32())
	if err != nil {
		panic(err)
	}
	return h2
}
