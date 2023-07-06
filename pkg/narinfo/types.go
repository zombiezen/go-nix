package narinfo

import (
	mh "github.com/multiformats/go-multihash/core"
	"github.com/nix-community/go-nix/nix"
	"github.com/nix-community/go-nix/pkg/hash"
	"github.com/nix-community/go-nix/pkg/narinfo/signature"
)

// NarInfo represents a parsed .narinfo file.
//
// Deprecated: Use [nix.NARInfo].
type NarInfo struct {
	StorePath string // The full nix store path (/nix/store/…-pname-version)

	URL         string // The relative location to the .nar[.xz,…] file. Usually nar/$fileHash.nar[.xz]
	Compression string // The compression method file at URL is compressed with (none,xz,…)

	FileHash *hash.Hash // The hash of the file at URL
	FileSize uint64     // The size of the file at URL, in bytes

	// The hash of the .nar file, after possible decompression
	// Identical to FileHash if no compression is used.
	NarHash *hash.Hash
	// The size of the .nar file, after possible decompression, in bytes.
	// Identical to FileSize if no compression is used.
	NarSize uint64

	// References to other store paths, contained in the .nar file
	References []string

	// Path of the .drv for this store path
	Deriver string

	// This doesn't seem to be used at all?
	System string

	// Signatures, if any.
	Signatures []signature.Signature

	// TODO: Figure out the meaning of this
	CA string
}

func (n *NarInfo) toNew() *nix.NARInfo {
	info := &nix.NARInfo{
		StorePath:   n.StorePath,
		URL:         n.URL,
		Compression: nix.CompressionType(n.Compression),
		FileHash:    toNewHash(n.FileHash),
		FileSize:    int64(n.FileSize),
		NARHash:     toNewHash(n.NarHash),
		NARSize:     int64(n.NarSize),
		System:      n.System,
		Deriver:     nix.ObjectName(n.Deriver),
	}
	for _, ref := range n.References {
		info.References = append(info.References, nix.ObjectName(ref))
	}
	for _, sig := range n.Signatures {
		sig2, err := nix.ParseSignature(sig.String())
		if err == nil {
			info.Sig = append(info.Sig, sig2)
		}
	}
	if n.CA != "" {
		info.CA, _ = nix.ParseContentAddress(n.CA)
	}
	return info
}

func (n *NarInfo) String() string {
	info := n.toNew()
	buf, err := info.MarshalText()
	if err != nil {
		panic(err)
	}
	return string(buf)
}

// ContentType returns the mime content type of the object.
func (n NarInfo) ContentType() string {
	return nix.NARInfoMIMEType
}

//nolint:gochecknoglobals
var mhTypeToHashType = map[int]nix.HashType{
	mh.MD5:      nix.MD5,
	mh.SHA1:     nix.SHA1,
	mh.SHA2_256: nix.SHA256,
	mh.SHA2_512: nix.SHA512,
}

func toNewHash(h *hash.Hash) nix.Hash {
	if h == nil {
		return nix.Hash{}
	}
	return nix.NewHash(mhTypeToHashType[h.HashType], h.Digest())
}
