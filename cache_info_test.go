package nix

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCacheInfoMarshalText(t *testing.T) {
	tests := []struct {
		info      *CacheInfo
		marshaled string
	}{
		{
			info:      new(CacheInfo),
			marshaled: "StoreDir: /nix/store\n",
		},
		{
			info:      &CacheInfo{StoreDirectory: "/foo"},
			marshaled: "StoreDir: /foo\n",
		},
		{
			info:      &CacheInfo{Priority: 40},
			marshaled: "StoreDir: /nix/store\nPriority: 40\n",
		},
		{
			info:      &CacheInfo{WantMassQuery: true},
			marshaled: "StoreDir: /nix/store\nWantMassQuery: 1\n",
		},
	}
	for _, test := range tests {
		got, err := test.info.MarshalText()
		if err != nil {
			t.Errorf("%#v.MarshalText(): %v", test.info, err)
			continue
		}
		if diff := cmp.Diff(test.marshaled, string(got)); diff != "" {
			t.Errorf("Marshal %#v (-want +got):\n%s", test.info, diff)
		}
	}
}

func TestCacheInfoUnmarshalText(t *testing.T) {
	tests := []struct {
		marshaled string
		want      *CacheInfo
	}{
		{
			marshaled: "",
			want:      &CacheInfo{StoreDirectory: "/nix/store"},
		},
		{
			marshaled: "StoreDir: /nix/store\n",
			want:      &CacheInfo{StoreDirectory: "/nix/store"},
		},
		{
			marshaled: "StoreDir: /foo\n",
			want:      &CacheInfo{StoreDirectory: "/foo"},
		},
		{
			marshaled: "StoreDir: /nix/store\nPriority: 40\n",
			want:      &CacheInfo{StoreDirectory: "/nix/store", Priority: 40},
		},
		{
			marshaled: "StoreDir: /nix/store\nWantMassQuery: 1\n",
			want:      &CacheInfo{StoreDirectory: "/nix/store", WantMassQuery: true},
		},
	}
	for _, test := range tests {
		got := new(CacheInfo)
		if err := got.UnmarshalText([]byte(test.marshaled)); err != nil {
			t.Errorf("new(CacheInfo).UnmarshalText(%q): %v", test.marshaled, err)
			continue
		}
		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("Unmarshal %q (-want +got):\n%s", test.marshaled, diff)
		}
	}
}
