package nix

import "testing"

const (
	nixosPublicKey = "cache.nixos.org-1:6NCHdD59X431o0gWypbMrAURkbJ16ZPMQFGspcDShjY="
	test1PublicKey = "test1:tLAEn+EeaBUJYqEpTd2yeerr7Ic6+0vWe+aXL/vYUpE="
	//nolint:gosec
	test1SecretKey = "test1:jbX9NxZp8WB/coK8k7yLf0gNYmBbIbCrOFwgJgI7OV+0sASf4R5oFQlioSlN3bJ56uvshzr7S9Z75pcv+9hSkQ=="
)

func TestPublicKey(t *testing.T) {
	tests := []struct {
		s    string
		name string
	}{
		{s: nixosPublicKey, name: "cache.nixos.org-1"},
		{s: test1PublicKey, name: "test1"},
	}
	for _, test := range tests {
		pub, err := ParsePublicKey(test.s)
		if err != nil {
			t.Errorf("ParsePublicKey(%q) = _, %v", test.s, err)
			continue
		}
		if got := pub.String(); got != test.s {
			t.Errorf("ParsePublicKey(%q).String() = %q; want %q", test.s, got, test.s)
		}
		if got := pub.Name(); got != test.name {
			t.Errorf("ParsePublicKey(%q).Name() = %q; want %q", test.s, got, test.name)
		}
	}
}

func TestPrivateKey(t *testing.T) {
	pk, err := ParsePrivateKey(test1SecretKey)
	if err != nil {
		t.Fatal(err)
	}
	if got := pk.String(); got != test1SecretKey {
		t.Errorf("ParsePrivateKey(%q).String() = %q; want %q", test1SecretKey, got, test1SecretKey)
	}
	if got, want := pk.Name(), "test1"; got != want {
		t.Errorf("ParsePrivateKey(%q).Name() = %q; want %q", test1SecretKey, got, want)
	}
	if got := pk.PublicKey().String(); got != test1PublicKey {
		t.Errorf("ParsePrivateKey(%q).PublicKey().String() = %q; want %q", test1SecretKey, got, test1PublicKey)
	}
}

func TestVerifyNARInfo(t *testing.T) {
	info := &NARInfo{
		StorePath: "/nix/store/syd87l2rxw8cbsxmxl853h0r6pdwhwjr-curl-7.82.0-bin",
		NARSize:   196040,
		References: []ObjectName{
			"0jqd0rlxzra1rs38rdxl43yh6rxchgc6-curl-7.82.0",
			"6w8g7njm4mck5dmjxws0z1xnrxvl81xa-glibc-2.34-115",
			"j5jxw3iy7bbz4a57fh9g2xm2gxmyal8h-zlib-1.2.12",
			"yxvjs9drzsphm9pcf42a4byzj1kb9m7k-openssl-1.1.1n",
		},
	}
	var err error
	info.NARHash, err = ParseHash("sha256:1b4sb93wp679q4zx9k1ignby1yna3z7c4c2ri3wphylbc2dwsys0")
	if err != nil {
		t.Fatal(err)
	}

	var trusted []*PublicKey
	pub, err := ParsePublicKey(nixosPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	trusted = append(trusted, pub)
	pub, err = ParsePublicKey(test1PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	trusted = append(trusted, pub)

	tests := []struct {
		sig  string
		want bool
	}{
		{"cache.nixos.org-1:TsTTb3WGTZKphvYdBHXwo6weVILmTytUjLB+vcX89fOjjRicCHmKA4RCPMVLkj6TMJ4GMX3HPVWRdD1hkeKZBQ==", true},
		{"test1:519iiVLx/c4Rdt5DNt6Y2Jm6hcWE9+XY69ygiWSZCNGVcmOcyL64uVAJ3cV8vaTusIZdbTnYo9Y7vDNeTmmMBQ==", true},
		{"test2:519iiVLx/c4Rdt5DNt6Y2Jm6hcWE9+XY69ygiWSZCNGVcmOcyL64uVAJ3cV8vaTusIZdbTnYo9Y7vDNeTmmMBQ==", false},
		{"test1:619iiVLx/c4Rdt5DNt6Y2Jm6hcWE9+XY69ygiWSZCNGVcmOcyL64uVAJ3cV8vaTusIZdbTnYo9Y7vDNeTmmMBQ==", false},
	}
	for _, test := range tests {
		sig, err := ParseSignature(test.sig)
		if err != nil {
			t.Errorf("ParseSignature(%q) = _, %v; want %s, <nil>", test.sig, err, test.sig)
			continue
		}
		if err := VerifyNARInfo(trusted, info, sig); test.want && err != nil {
			t.Errorf("VerifyNARInfo(%v, ..., %v) = %v; want <nil>", trusted, sig, err)
		} else if !test.want && err == nil {
			t.Errorf("VerifyNARInfo(%v, ..., %v) = <nil>; want error", trusted, sig)
		}
	}
}

func TestSignNARInfo(t *testing.T) {
	pk, err := ParsePrivateKey(test1SecretKey)
	if err != nil {
		t.Fatal(err)
	}
	info := &NARInfo{
		StorePath: "/nix/store/syd87l2rxw8cbsxmxl853h0r6pdwhwjr-curl-7.82.0-bin",
		NARSize:   196040,
		References: []ObjectName{
			"0jqd0rlxzra1rs38rdxl43yh6rxchgc6-curl-7.82.0",
			"6w8g7njm4mck5dmjxws0z1xnrxvl81xa-glibc-2.34-115",
			"j5jxw3iy7bbz4a57fh9g2xm2gxmyal8h-zlib-1.2.12",
			"yxvjs9drzsphm9pcf42a4byzj1kb9m7k-openssl-1.1.1n",
		},
	}
	info.NARHash, err = ParseHash("sha256:1b4sb93wp679q4zx9k1ignby1yna3z7c4c2ri3wphylbc2dwsys0")
	if err != nil {
		t.Fatal(err)
	}

	sig, err := SignNARInfo(pk, info)
	if err != nil {
		t.Fatal(err)
	}
	const wantSig = "test1:519iiVLx/c4Rdt5DNt6Y2Jm6hcWE9+XY69ygiWSZCNGVcmOcyL64uVAJ3cV8vaTusIZdbTnYo9Y7vDNeTmmMBQ=="
	if got := sig.String(); got != wantSig {
		t.Errorf("SignNARInfo(%v, info) = %v, <nil>; want %v, <nil>", pk, got, wantSig)
	}
}
