package narinfo

import "strings"

// Fingerprint is the digest that will be used with a private key to generate
// one of the signatures.
func (n NarInfo) Fingerprint() string {
	sb := new(strings.Builder)
	if err := n.toNew().WriteFingerprint(sb); err != nil {
		panic(err)
	}
	return sb.String()
}
