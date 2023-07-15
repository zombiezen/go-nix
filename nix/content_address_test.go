package nix

import (
	"testing"

	"github.com/nix-community/go-nix/nixbase32"
)

const testSHA256Base32 = "1b8m03r63zqhnjf7l5wnldhh7c134ap5vpj0850ymkq1iyzicy5s"

func TestParseContentAddress(t *testing.T) {
	sha256Bits, err := nixbase32.DecodeString(testSHA256Base32)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		s    string
		want ContentAddress
		err  bool
	}{
		{
			s:    "text:sha256:" + testSHA256Base32,
			want: TextContentAddress(NewHash(SHA256, sha256Bits)),
		},
		{
			s:    "fixed:sha256:" + testSHA256Base32,
			want: FlatFileContentAddress(NewHash(SHA256, sha256Bits)),
		},
		{
			s:    "fixed:r:sha256:" + testSHA256Base32,
			want: RecursiveFileContentAddress(NewHash(SHA256, sha256Bits)),
		},
		{
			s:   "fixed:r:sha256-" + testSHA256Base32,
			err: true,
		},
	}
	for _, test := range tests {
		got, err := ParseContentAddress(test.s)
		if !got.Equal(test.want) || (err != nil) != test.err {
			errString := "<nil>"
			if test.err {
				errString = "<error>"
			}
			t.Errorf("ParseContentAddress(%q) = %v, %v; want %v, %s",
				test.s, got, err, test.want, errString)
		}
	}

	t.Run("UnmarshalText", func(t *testing.T) {
		for _, test := range tests {
			var got ContentAddress
			err := got.UnmarshalText([]byte(test.s))
			if !got.Equal(test.want) || (err != nil) != test.err {
				errString := "<nil>"
				if test.err {
					errString = "<error>"
				}
				t.Errorf("new(ContentAddress).UnmarshalText(%q) = %v, %v; want %v, %s",
					test.s, got, err, test.want, errString)
			}
		}
	})
}

func TestContentAddressString(t *testing.T) {
	sha256Bits, err := nixbase32.DecodeString(testSHA256Base32)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		ca              ContentAddress
		zero            bool
		isText          bool
		isFixed         bool
		isRecursiveFile bool
		hash            Hash
		wantString      string
	}{
		{
			ca:         ContentAddress{},
			zero:       true,
			wantString: "",
		},
		{
			ca:         TextContentAddress(NewHash(SHA256, sha256Bits)),
			isText:     true,
			hash:       NewHash(SHA256, sha256Bits),
			wantString: "text:sha256:" + testSHA256Base32,
		},
		{
			ca:         FlatFileContentAddress(NewHash(SHA256, sha256Bits)),
			isFixed:    true,
			hash:       NewHash(SHA256, sha256Bits),
			wantString: "fixed:sha256:" + testSHA256Base32,
		},
		{
			ca:              RecursiveFileContentAddress(NewHash(SHA256, sha256Bits)),
			isFixed:         true,
			isRecursiveFile: true,
			hash:            NewHash(SHA256, sha256Bits),
			wantString:      "fixed:r:sha256:" + testSHA256Base32,
		},
	}

	t.Run("IsZero", func(t *testing.T) {
		for _, test := range tests {
			got := test.ca.IsZero()
			if got != test.zero {
				t.Errorf("(%#v).IsZero() = %t; want %t", test.ca, got, test.zero)
			}
		}
	})

	t.Run("IsText", func(t *testing.T) {
		for _, test := range tests {
			got := test.ca.IsText()
			if got != test.isText {
				t.Errorf("(%#v).IsText() = %t; want %t", test.ca, got, test.isText)
			}
		}
	})

	t.Run("IsFixed", func(t *testing.T) {
		for _, test := range tests {
			got := test.ca.IsFixed()
			if got != test.isFixed {
				t.Errorf("(%#v).IsFixed() = %t; want %t", test.ca, got, test.isFixed)
			}
		}
	})

	t.Run("IsRecursiveFile", func(t *testing.T) {
		for _, test := range tests {
			got := test.ca.IsRecursiveFile()
			if got != test.isRecursiveFile {
				t.Errorf("(%#v).IsRecursiveFile() = %t; want %t", test.ca, got, test.isRecursiveFile)
			}
		}
	})

	t.Run("Hash", func(t *testing.T) {
		for _, test := range tests {
			got := test.ca.Hash()
			if !got.Equal(test.hash) {
				t.Errorf("(%#v).Hash() = %v; want %v", test.ca, got, test.hash)
			}
		}
	})

	t.Run("String", func(t *testing.T) {
		for _, test := range tests {
			got := test.ca.String()
			if got != test.wantString {
				t.Errorf("(%#v).String() = %q; want %q", test.ca, got, test.wantString)
			}
		}
	})

	t.Run("MarshalText", func(t *testing.T) {
		for _, test := range tests {
			got, err := test.ca.MarshalText()
			switch {
			case test.zero && err == nil:
				t.Errorf("(%#v).MarshalText() = %q, <nil>; want _, <error>", test.ca, got)
			case !test.zero && (string(got) != test.wantString || err != nil):
				t.Errorf("(%#v).MarshalText() = %q, %v; want %q, <nil>", test.ca, got, err, test.wantString)
			}
		}
	})
}
