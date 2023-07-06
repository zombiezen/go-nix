package nix

import (
	"bytes"
	"fmt"
	"io"
	slashpath "path"
	"sort"
	"strconv"
)

// NARInfoExtension is the file extension for a file containing NAR information.
const NARInfoExtension = ".narinfo"

// NARInfoMIMEType is the MIME content type for a .narinfo file.
const NARInfoMIMEType = "text/x-nix-narinfo"

// NARInfo represents a parsed .narinfo file.
type NARInfo struct {
	// StorePath is the absolute path of this store object
	// (e.g. "/nix/store/s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1").
	// Nix requires this field to be set.
	StorePath string
	// URL is the path to download to the (possibly compressed) .nar file,
	// relative to the .narinfo file's directory.
	// Nix requires this field to be set.
	URL string
	// Compression is the algorithm used for the file referenced by URL.
	// If empty, defaults to "bzip2".
	Compression CompressionType
	// FileHash is the hash of the file referenced by URL.
	// If Compression is [NoCompression] and FileHash is the zero hash,
	// then NARHash will be used.
	FileHash Hash
	// FileSize is the size of the file referenced by URL in bytes.
	// If Compression is [NoCompression] and FileSize is zero,
	// then NARSize will be used.
	FileSize int64
	// NARHash is the hash of the decompressed .nar file.
	// Nix requires this field to be set.
	NARHash Hash
	// NARSize is the size of the decompressed .nar file in bytes.
	// Nix requires this field to be set.
	NARSize int64
	// References is the set of other store objects that this store object references.
	References []ObjectName
	// Deriver is the name of the store object that is the store derivation
	// of this store object.
	Deriver ObjectName
	// System is a deprecated field.
	//
	// Deprecated: Ignore this field.
	System string
	// Sig is a set of signatures for this object.
	Sig []*Signature
	// CA is an optional content-addressability assertion.
	CA ContentAddress
}

// Clone returns a deep copy of an info struct.
func (info *NARInfo) Clone() *NARInfo {
	info2 := new(NARInfo)
	*info2 = *info
	info.References = append([]ObjectName(nil), info.References...)
	info.Sig = append([]*Signature(nil), info.Sig...)
	return info
}

// Directory returns the store directory of the store object.
func (info *NARInfo) StoreDirectory() StoreDirectory {
	return StoreDirectory(slashpath.Dir(info.StorePath))
}

// ObjectName returns the base name of the store path.
func (info *NARInfo) ObjectName() ObjectName {
	_, name, _ := ParseStorePath(info.StorePath)
	return name
}

// IsValid reports whether the NAR information fields are valid.
func (info *NARInfo) IsValid() bool {
	return info.validate() == nil
}

// AddSignatures adds signatures that are not already present in info.
func (info *NARInfo) AddSignatures(sigs ...*Signature) {
addLoop:
	for _, newSig := range sigs {
		for _, oldSig := range info.Sig {
			if oldSig.String() == newSig.String() {
				continue addLoop
			}
		}
		info.Sig = append(info.Sig, newSig)
	}
}

// validateFingerprint validates the subset of fields needed for [NARInfo.WriteFingerprint].
func (info *NARInfo) validateForFingerprint() error {
	if info.StorePath == "" {
		return fmt.Errorf("store path empty")
	}
	if _, _, err := ParseStorePath(info.StorePath); err != nil {
		return fmt.Errorf("store path: %v", err)
	}
	if info.NARHash.IsZero() {
		return fmt.Errorf("nar hash not set")
	}
	if info.NARSize == 0 {
		return fmt.Errorf("nar size not set")
	}
	if info.NARSize < 0 {
		return fmt.Errorf("negative nar size")
	}
	return nil
}

func (info *NARInfo) validate() error {
	if err := info.validateForFingerprint(); err != nil {
		return err
	}
	if info.URL == "" {
		return fmt.Errorf("url empty")
	}
	if !info.Compression.IsKnown() {
		return fmt.Errorf("unknown compression %q", info.Compression)
	}

	if info.FileSize < 0 {
		return fmt.Errorf("negative file size")
	}
	if info.Compression == NoCompression {
		if info.FileSize != 0 && info.FileSize != info.NARSize {
			return fmt.Errorf("compression = %q and file size (%d) != nar size (%d)",
				NoCompression, info.FileSize, info.NARSize)
		}
		if !info.FileHash.IsZero() && !info.FileHash.Equal(info.NARHash) {
			return fmt.Errorf("compression = %q and file hash (%v) != nar hash (%v)",
				NoCompression, info.FileHash, info.NARHash)
		}
	}

	return nil
}

// WriteFingerprint writes the store object's "fingerprint" to the given writer.
// The fingerprint is the string used for signing.
func (info *NARInfo) WriteFingerprint(w io.Writer) error {
	if err := info.validateForFingerprint(); err != nil {
		return fmt.Errorf("compute nix store object fingerprint: %v", err)
	}

	if _, err := io.WriteString(w, "1;"); err != nil {
		return fmt.Errorf("compute nix store object fingerprint for %s: %w", info.StorePath, err)
	}
	if _, err := io.WriteString(w, info.StorePath); err != nil {
		return fmt.Errorf("compute nix store object fingerprint for %s: %w", info.StorePath, err)
	}
	if _, err := io.WriteString(w, ";"); err != nil {
		return fmt.Errorf("compute nix store object fingerprint for %s: %w", info.StorePath, err)
	}
	if _, err := io.WriteString(w, info.NARHash.Base32()); err != nil {
		return fmt.Errorf("compute nix store object fingerprint for %s: %w", info.StorePath, err)
	}
	if _, err := io.WriteString(w, ";"); err != nil {
		return fmt.Errorf("compute nix store object fingerprint for %s: %w", info.StorePath, err)
	}
	if _, err := io.WriteString(w, strconv.FormatInt(info.NARSize, 10)); err != nil {
		return fmt.Errorf("compute nix store object fingerprint for %s: %w", info.StorePath, err)
	}
	if _, err := io.WriteString(w, ";"); err != nil {
		return fmt.Errorf("compute nix store object fingerprint for %s: %w", info.StorePath, err)
	}

	sortedRefs := append([]ObjectName(nil), info.References...)
	sort.Slice(sortedRefs, func(i, j int) bool {
		return sortedRefs[i] < sortedRefs[j]
	})
	storeDir := info.StoreDirectory()
	for i, ref := range sortedRefs {
		if i > 0 {
			if ref == sortedRefs[i-1] {
				// Deduplicate on-the-fly.
				continue
			}
			if _, err := io.WriteString(w, ","); err != nil {
				return fmt.Errorf("compute nix store object fingerprint for %s: %w", info.StorePath, err)
			}
		}
		if _, err := io.WriteString(w, storeDir.Path(ref)); err != nil {
			return fmt.Errorf("compute nix store object fingerprint for %s: %w", info.StorePath, err)
		}
	}

	return nil
}

// UnmarshalText decodes a .narinfo file.
func (info *NARInfo) UnmarshalText(src []byte) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("unmarshal narinfo: %v", err)
		}
	}()

	newline := []byte("\n")
	*info = NARInfo{}
	hasReferences := false
	for lineno := 1; len(src) > 0; lineno++ {
		i := bytes.IndexByte(src, ':')
		if i < 0 {
			return fmt.Errorf("line %d: could not find ':'", lineno)
		}
		key := string(src[:i])
		lineno += bytes.Count(src[:i+len(": ")], newline)
		src = src[i+len(": "):]

		i = bytes.IndexByte(src, '\n')
		if i < 0 {
			return fmt.Errorf("line %d: missing newline", lineno)
		}
		value := src[:i]
		src = src[i+1:]

		switch key {
		case "StorePath":
			if info.StorePath != "" {
				return fmt.Errorf("line %d: duplicate StorePath", lineno)
			}
			info.StorePath = string(value)
			if info.StorePath == "" {
				return fmt.Errorf("line %d: empty StorePath", lineno)
			}
			if !slashpath.IsAbs(info.StorePath) {
				return fmt.Errorf("line %d: store path %q not absolute", lineno, info.StorePath)
			}
		case "URL":
			if info.URL != "" {
				return fmt.Errorf("line %d: duplicate URL", lineno)
			}
			info.URL = string(value)
		case "Compression":
			if info.Compression != "" {
				return fmt.Errorf("line %d: duplicate Compression", lineno)
			}
			info.Compression = CompressionType(value)
			if info.Compression == "" {
				return fmt.Errorf("line %d: empty Compression", lineno)
			}
			if !info.Compression.IsKnown() {
				return fmt.Errorf("line %d: unknown compression %q", lineno, info.Compression)
			}
		case "FileHash":
			if !info.FileHash.IsZero() {
				return fmt.Errorf("line %d: duplicate FileHash", lineno)
			}
			if err := info.FileHash.UnmarshalText(value); err != nil {
				return fmt.Errorf("line %d: FileHash: %v", lineno, err)
			}
		case "FileSize":
			if info.FileSize > 0 {
				return fmt.Errorf("line %d: duplicate FileSize", lineno)
			}
			var err error
			info.FileSize, err = strconv.ParseInt(string(value), 10, 64)
			if err != nil {
				return fmt.Errorf("line %d: FileSize: %v", lineno, err)
			}
			if info.FileSize <= 0 {
				return fmt.Errorf("line %d: FileSize is non-positive", lineno)
			}
		case "NarHash":
			if !info.NARHash.IsZero() {
				return fmt.Errorf("line %d: duplicate NarHash", lineno)
			}
			if err := info.NARHash.UnmarshalText(value); err != nil {
				return fmt.Errorf("line %d: NarHash: %v", lineno, err)
			}
		case "NarSize":
			if info.NARSize > 0 {
				return fmt.Errorf("line %d: duplicate NarSize", lineno)
			}
			var err error
			info.NARSize, err = strconv.ParseInt(string(value), 10, 64)
			if err != nil {
				return fmt.Errorf("line %d: NarSize: %v", lineno, err)
			}
			if info.NARSize <= 0 {
				return fmt.Errorf("line %d: NarSize is non-positive", lineno)
			}
		case "References":
			if hasReferences {
				return fmt.Errorf("line %d: duplicate References", lineno)
			}
			hasReferences = true
			words := bytes.Fields(value)
			info.References = make([]ObjectName, 0, len(words))
			for _, w := range words {
				name, err := ParseObjectName(string(w))
				if err != nil {
					return fmt.Errorf("line %d: References: %v", lineno, err)
				}
				info.References = append(info.References, name)
			}
		case "Deriver":
			if info.Deriver != "" {
				return fmt.Errorf("line %d: duplicate Deriver", lineno)
			}
			var err error
			info.Deriver, err = ParseObjectName(string(value))
			if err != nil {
				return fmt.Errorf("line %d: Deriver: %v", lineno, err)
			}
		case "System":
			if info.System != "" {
				return fmt.Errorf("line %d: duplicate System", lineno)
			}
			info.System = string(value)
		case "Sig":
			sig := new(Signature)
			if err := sig.UnmarshalText(value); err != nil {
				return fmt.Errorf("line %d: Sig: %v", lineno, err)
			}
			info.Sig = append(info.Sig, sig)
		case "CA":
			if !info.CA.IsZero() {
				return fmt.Errorf("line %d: duplicate CA", lineno)
			}
			if err := info.CA.UnmarshalText(value); err != nil {
				return fmt.Errorf("line %d: CA: %v", lineno, err)
			}
		}
	}

	if info.Compression == "" {
		info.Compression = Bzip2
	}
	if info.Compression == NoCompression {
		if info.FileHash.IsZero() {
			info.FileHash = info.NARHash
		}
		if info.FileSize == 0 {
			info.FileSize = info.NARSize
		}
	}

	return info.validate()
}

// MarshalText encodes the information as a .narinfo file.
func (info *NARInfo) MarshalText() ([]byte, error) {
	if err := info.validate(); err != nil {
		return nil, fmt.Errorf("marshal narinfo: %v", err)
	}

	var buf []byte
	buf = append(buf, "StorePath: "...)
	buf = append(buf, info.StorePath...)
	buf = append(buf, "\nURL: "...)
	buf = append(buf, info.URL...)
	buf = append(buf, "\nCompression: "...)
	compression := info.Compression
	if compression == "" {
		compression = Bzip2
	}
	buf = append(buf, compression...)
	if !info.FileHash.IsZero() {
		buf = append(buf, "\nFileHash: "...)
		buf = append(buf, info.FileHash.Base32()...)
	}
	if info.FileSize != 0 {
		buf = append(buf, "\nFileSize: "...)
		buf = strconv.AppendInt(buf, info.FileSize, 10)
	}
	buf = append(buf, "\nNarHash: "...)
	buf = append(buf, info.NARHash.Base32()...)
	buf = append(buf, "\nNarSize: "...)
	buf = strconv.AppendInt(buf, info.NARSize, 10)
	if len(info.References) > 0 {
		buf = append(buf, "\nReferences:"...)
		for _, ref := range info.References {
			buf = append(buf, ' ')
			buf = append(buf, ref...)
		}
	}
	if info.Deriver != "" {
		buf = append(buf, "\nDeriver: "...)
		buf = append(buf, info.Deriver...)
	}
	if info.System != "" {
		buf = append(buf, "\nSystem: "...)
		buf = append(buf, info.System...)
	}
	for _, sig := range info.Sig {
		buf = append(buf, "\nSig: "...)
		sigData, err := sig.MarshalText()
		if err != nil {
			return nil, fmt.Errorf("marshal narinfo: %v", err)
		}
		buf = append(buf, sigData...)
	}
	if !info.CA.IsZero() {
		buf = append(buf, "\nCA: "...)
		caData, err := info.CA.MarshalText()
		if err != nil {
			return nil, fmt.Errorf("marshal narinfo: %v", err)
		}
		buf = append(buf, caData...)
	}
	buf = append(buf, "\n"...)
	return buf, nil
}

// CompressionType is an enumeration of compression algorithms used in [NARInfo].
type CompressionType string

// Compression types.
const (
	NoCompression CompressionType = "none"
	Gzip          CompressionType = "gzip"
	Bzip2         CompressionType = "bzip2"
	XZ            CompressionType = "xz"
	Zstandard     CompressionType = "zstd"
	Lzip          CompressionType = "lzip"
	LZ4           CompressionType = "lz4"
	Brotli        CompressionType = "br"
)

// IsKnown reports whether ct is one of the known compression types.
func (ct CompressionType) IsKnown() bool {
	switch ct {
	case "", NoCompression, Gzip, Bzip2, XZ, Zstandard, Lzip, LZ4, Brotli:
		return true
	default:
		return false
	}
}
