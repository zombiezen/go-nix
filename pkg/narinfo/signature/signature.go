// Package signature provides types for signing keys.
//
// Deprecated: Use [github.com/nix-community/go-nix/nix.Signature],
// [github.com/nix-community/go-nix/nix.PublicKey],
// or [github.com/nix-community/go-nix/nix.PrivateKey]
// rather than the types in this package.
package signature

import (
	"crypto/ed25519"
	"fmt"
)

// Signature represents a named ed25519 signature.
//
// Deprecated: Use [github.com/nix-community/go-nix/nix.Signature] instead.
type Signature struct {
	Name string
	Data []byte
}

// String returns the encoded <keyname>:<base64-signature-data>.
func (s Signature) String() string {
	return encode(s.Name, s.Data)
}

// ParseSignature decodes a <keyname>:<base64-signature-data>
// and returns a *Signature, or an error.
//
// Deprecated: Use [github.com/nix-community/go-nix/nix.ParseSignature] instead.
func ParseSignature(s string) (Signature, error) {
	name, data, err := decode(s, ed25519.SignatureSize)
	if err != nil {
		return Signature{}, fmt.Errorf("signature is corrupt: %w", err)
	}

	return Signature{name, data}, nil
}

// VerifyFirst returns the result of the first signature that matches a public
// key. If no matching public key was found, it returns false.
//
// Deprecated: Use [github.com/nix-community/go-nix/nix.VerifyNARInfo] instead.
func VerifyFirst(fingerprint string, signatures []Signature, pubKeys []PublicKey) bool {
	for _, key := range pubKeys {
		for _, sig := range signatures {
			if key.Name == sig.Name {
				return key.Verify(fingerprint, sig)
			}
		}
	}

	return false
}
