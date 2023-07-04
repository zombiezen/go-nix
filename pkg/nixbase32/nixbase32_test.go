package nixbase32

import (
	"bytes"
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

//nolint:gochecknoglobals
var tests = []struct {
	dec []byte
	enc string
}{
	{[]byte{}, ""},
	{[]byte{0x1f}, "0z"},
	{
		[]byte{
			0xd8, 0x6b, 0x33, 0x92, 0xc1, 0x20, 0x2e, 0x8f,
			0xf5, 0xa4, 0x23, 0xb3, 0x02, 0xe6, 0x28, 0x4d,
			0xb7, 0xf8, 0xf4, 0x35, 0xea, 0x9f, 0x39, 0xb5,
			0xb1, 0xb2, 0x0f, 0xd3, 0xac, 0x36, 0xdf, 0xcb,
		},
		"1jyz6snd63xjn6skk7za6psgidsd53k05cr3lksqybi0q6936syq",
	},
}

func TestEncodeToString(t *testing.T) {
	for _, test := range tests {
		if got := EncodeToString(test.dec); got != test.enc {
			t.Errorf("EncodeToString(%q) = %q; want %q", test.dec, got, test.enc)
		}
	}
}

func TestDecodeString(t *testing.T) {
	for _, test := range tests {
		got, err := DecodeString(test.enc)
		if err != nil || !bytes.Equal(got, test.dec) {
			t.Errorf("DecodeString(%q) = %02x, %v; want %02x, <nil>", test.enc, got, err, test.dec)
		}
	}
	invalidEncodings := []string{
		// invalid character
		"0t",
		// this is invalid encoding, because it encodes 10 1-bytes, so the carry
		// would be 2 1-bytes
		"zz",
		// this is an even more specific example - it'd decode as 00000000 11
		"c0",
	}
	for _, bad := range invalidEncodings {
		if got, err := DecodeString(bad); err == nil {
			t.Errorf("DecodeString(%q) = %q, <nil>; want _, <error>", bad, got)
		}
	}
}

func TestIs(t *testing.T) {
	for c := int16(0); c <= 0xff; c++ {
		got := Is(byte(c))
		want := strings.IndexByte(alphabet, byte(c)) != -1
		if got != want {
			t.Errorf("Is(%q) = %t; want %t", byte(c), got, want)
		}
	}
}

func BenchmarkEncode(b *testing.B) {
	sizes := []int{32, 64, 128}

	for _, s := range sizes {
		bytes := make([]byte, s)
		rand.Read(bytes) //nolint:gosec

		b.Run(strconv.Itoa(s), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				EncodeToString(bytes)
			}
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	sizes := []int{32, 64, 128}

	for _, s := range sizes {
		bytes := make([]byte, s)
		rand.Read(bytes) //nolint:gosec
		input := EncodeToString(bytes)

		b.Run(strconv.Itoa(s), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := DecodeString(input)
				if err != nil {
					b.Fatal("error: %w", err)
				}
			}
		})
	}
}
