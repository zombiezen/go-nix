package nix

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io"
	"unicode"
)

func marshalKey(name string, data []byte) []byte {
	buf := make([]byte, 0, len(name)+len(":")+base64.StdEncoding.EncodedLen(len(data)))
	buf = append(buf, name...)
	buf = append(buf, ':')
	base64.StdEncoding.Encode(buf[len(buf):cap(buf)], data)
	return buf[:cap(buf)]
}

func unmarshalKey(s []byte, wantDataSize int) (name string, data []byte, err error) {
	nameBytes, base64Data, ok := bytes.Cut(s, []byte(":"))
	if !ok {
		return "", nil, fmt.Errorf("missing ':'")
	}
	if len(nameBytes) == 0 {
		return "", nil, fmt.Errorf("name is empty")
	}
	if bytes.IndexFunc(nameBytes, unicode.IsSpace) != -1 {
		return "", nil, fmt.Errorf("name %q contains spaces", nameBytes)
	}

	data = make([]byte, base64.StdEncoding.DecodedLen(len(base64Data)))
	n, err := base64.StdEncoding.Decode(data, base64Data)
	if err != nil {
		return "", nil, err
	}
	data = data[:n]
	if len(data) != wantDataSize {
		return "", nil, fmt.Errorf("expected %d base64 characters (got %d)",
			base64.StdEncoding.EncodedLen(wantDataSize), len(base64Data))
	}
	return string(nameBytes), data, nil
}

// A PublicKey is a Nix public signing key.
type PublicKey struct {
	name string
	data ed25519.PublicKey
}

// ParsePublicKey parses the string encoding of a public key.
// It is a wrapper around [PublicKey.UnmarshalText].
func ParsePublicKey(s string) (*PublicKey, error) {
	pub := new(PublicKey)
	if err := pub.UnmarshalText([]byte(s)); err != nil {
		return nil, err
	}
	return pub, nil
}

// Name returns the public key's identifier.
func (pub *PublicKey) Name() string {
	return pub.name
}

// String formats the public key as "<name>:<base64 data>".
func (pub *PublicKey) String() string {
	return string(marshalKey(pub.name, pub.data))
}

// MarshalText formats the public key as "<name>:<base64 data>".
// It returns an error if pub is nil.
func (pub *PublicKey) MarshalText() ([]byte, error) {
	if pub == nil {
		return nil, fmt.Errorf("marshal nix public key: cannot marshal nil")
	}
	return marshalKey(pub.name, pub.data), nil
}

// UnmarshalText parses the string encoding of a public key.
func (pub *PublicKey) UnmarshalText(data []byte) error {
	var err error
	pub.name, pub.data, err = unmarshalKey(data, ed25519.PublicKeySize)
	if err != nil {
		return fmt.Errorf("unmarshal nix public key %q: %v", data, err)
	}
	return nil
}

// A PrivateKey is a Nix private signing key.
// It is used to produce a [Signature]
// for a Nix store object (represented by [NARInfo]).
type PrivateKey struct {
	name string
	data ed25519.PrivateKey
}

// GenerateKey generates a Nix signing key using entropy from rand.
// If rand is nil, [crypto/rand.Reader] will be used.
func GenerateKey(name string, rand io.Reader) (*PublicKey, *PrivateKey, error) {
	pub, pk, err := ed25519.GenerateKey(rand)
	if err != nil {
		return nil, nil, fmt.Errorf("generate nix signing key: %v", err)
	}
	return &PublicKey{name: name, data: pub}, &PrivateKey{name: name, data: pk}, nil
}

// ParsePrivateKey parses the string encoding of a private key.
// It is a wrapper around [PrivateKey.UnmarshalText].
func ParsePrivateKey(s string) (*PrivateKey, error) {
	pk := new(PrivateKey)
	if err := pk.UnmarshalText([]byte(s)); err != nil {
		return nil, err
	}
	return pk, nil
}

// Name returns the private key's identifier.
func (pk *PrivateKey) Name() string {
	return pk.name
}

// String formats the private key as "<name>:<base64 data>".
func (pk *PrivateKey) String() string {
	return string(marshalKey(pk.name, pk.data))
}

// PublicKey returns the public portion of the private key.
func (pk *PrivateKey) PublicKey() *PublicKey {
	return &PublicKey{
		name: pk.name,
		data: pk.data.Public().(ed25519.PublicKey),
	}
}

// MarshalText formats the private key as "<name>:<base64 data>".
// It returns an error if pub is nil.
func (pk *PrivateKey) MarshalText() ([]byte, error) {
	if pk == nil {
		return nil, fmt.Errorf("marshal nix private key: cannot marshal nil")
	}
	return marshalKey(pk.name, pk.data), nil
}

// UnmarshalText parses the string encoding of a private key.
func (pk *PrivateKey) UnmarshalText(data []byte) error {
	var err error
	pk.name, pk.data, err = unmarshalKey(data, ed25519.PrivateKeySize)
	if err != nil {
		return fmt.Errorf("unmarshal nix private key: %v", err)
	}
	return nil
}

// SignNARInfo signs the given [NARInfo] with the private key.
func SignNARInfo(pk *PrivateKey, info *NARInfo) (*Signature, error) {
	buf := new(bytes.Buffer)
	if err := info.WriteFingerprint(buf); err != nil {
		return nil, fmt.Errorf("sign %s with %s: %v", info.StorePath, pk.name, err)
	}
	sig, err := pk.data.Sign(nil, buf.Bytes(), crypto.Hash(0))
	if err != nil {
		return nil, fmt.Errorf("sign %s with %s: %v", info.StorePath, pk.name, err)
	}
	return &Signature{
		name: pk.name,
		data: sig,
	}, nil
}

// A Signature is a signature of a Nix store object
// created by a [PrivateKey].
type Signature struct {
	name string
	data []byte
}

// ParseSignature parses the string encoding of a signature.
// It is a wrapper around [Signature.UnmarshalText].
func ParseSignature(s string) (*Signature, error) {
	pub := new(Signature)
	if err := pub.UnmarshalText([]byte(s)); err != nil {
		return nil, err
	}
	return pub, nil
}

// Name returns the identifier of the [PrivateKey] used to produce this signature.
func (sig *Signature) Name() string {
	return sig.name
}

// String formats the signature as "<key name>:<base64 data>".
func (sig *Signature) String() string {
	return string(marshalKey(sig.name, sig.data))
}

// MarshalText formats the signature as "<key name>:<base64 data>".
// It returns an error if pub is nil.
func (sig *Signature) MarshalText() ([]byte, error) {
	if sig == nil {
		return nil, fmt.Errorf("marshal nix signature: cannot marshal nil")
	}
	return marshalKey(sig.name, sig.data), nil
}

// UnmarshalText parses the string encoding of a signature.
func (sig *Signature) UnmarshalText(data []byte) error {
	var err error
	sig.name, sig.data, err = unmarshalKey(data, ed25519.SignatureSize)
	if err != nil {
		return fmt.Errorf("unmarshal nix signature %q: %v", data, err)
	}
	return nil
}

// VerifyNARInfo verifies that a signature for a [NARInfo]
// matches the signature of the same name in a list of trusted keys.
// The trusted key list should not contain more than one key with the same name.
func VerifyNARInfo(trusted []*PublicKey, info *NARInfo, sig *Signature) error {
	if info.StorePath == "" {
		return fmt.Errorf("verify nar info: empty store path")
	}
	var foundPub *PublicKey
	for _, pub := range trusted {
		if pub.Name() == sig.Name() {
			foundPub = pub
			break
		}
	}
	if foundPub == nil {
		return fmt.Errorf("verify %s: key %s unknown", info.StorePath, sig.Name())
	}

	buf := new(bytes.Buffer)
	if err := info.WriteFingerprint(buf); err != nil {
		return fmt.Errorf("verify %s: %v", info.StorePath, err)
	}
	if !ed25519.Verify(foundPub.data, buf.Bytes(), sig.data) {
		return fmt.Errorf("verify %s: signature for key %s is invalid", info.StorePath, sig.Name())
	}
	return nil
}
