package nar_test

import (
	"fmt"
	"testing"

	"github.com/nix-community/go-nix/pkg/nar"
)

//nolint:gochecknoglobals
var cases = []struct {
	path1    string
	path2    string
	expected bool
}{
	{
		path1:    "/foo",
		path2:    "/foo",
		expected: true,
	},
	{
		path1:    "/fooa",
		path2:    "/foob",
		expected: true,
	},
	{
		path1:    "/foob",
		path2:    "/fooa",
		expected: false,
	},
	{
		path1:    "/cmd/structlayout/main.go",
		path2:    "/cmd/structlayout-optimize",
		expected: true,
	},
	{
		path1:    "/cmd/structlayout-optimize",
		path2:    "/cmd/structlayout-ao/main.go",
		expected: false,
	},
}

func TestLexicographicallyOrdered(t *testing.T) {
	for _, test := range cases {
		got := nar.PathIsLexicographicallyOrdered(test.path1, test.path2)
		if got != test.expected {
			t.Errorf("PathIsLexicographicallyOrdered(%q, %q) = %t; want %t", test.path1, test.path2, got, test.expected)
		}
	}
}

func BenchmarkLexicographicallyOrdered(b *testing.B) {
	for i, testCase := range cases {
		b.Run(fmt.Sprint(i), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				nar.PathIsLexicographicallyOrdered(testCase.path1, testCase.path2)
			}
		})
	}
}
