package nix

import (
	"bytes"
	"fmt"
	"strconv"
)

// CacheInfoName is the name of the binary cache resource
// that contains its [CacheInfo].
const CacheInfoName = "nix-cache-info"

// CacheInfoMIMEType is the MIME content type for the nix-cache-info file.
const CacheInfoMIMEType = "text/x-nix-cache-info"

// CacheInfo holds various settings about a Nix binary cache.
type CacheInfo struct {
	// StoreDirectory is the location of the store.
	// Defaults to [DefaultStoreDirectory] if empty.
	StoreDirectory StoreDirectory
	// Priority is the priority of the store when used as a substituter.
	// Lower values mean higher priority.
	Priority int
	// WantMassQuery indicates whether this store (when used as a substituter)
	// can be queried efficiently for path validity.
	WantMassQuery bool
}

// MarshalText formats the binary cache information in the format of a nix-cache-info file.
func (info *CacheInfo) MarshalText() ([]byte, error) {
	storeDir := info.StoreDirectory
	if storeDir == "" {
		storeDir = DefaultStoreDirectory
	}
	var buf []byte
	buf = append(buf, "StoreDir: "...)
	buf = append(buf, storeDir...)
	buf = append(buf, '\n')
	if info.Priority != 0 {
		buf = append(buf, "Priority: "...)
		buf = strconv.AppendInt(buf, int64(info.Priority), 10)
		buf = append(buf, '\n')
	}
	if info.WantMassQuery {
		buf = append(buf, "WantMassQuery: 1\n"...)
	}
	return buf, nil
}

// UnmarshalText parses the binary cache information from a nix-cache-info file.
func (info *CacheInfo) UnmarshalText(data []byte) error {
	*info = CacheInfo{StoreDirectory: DefaultStoreDirectory}
	for lineIdx, line := range bytes.Split(data, []byte{'\n'}) {
		lineno := lineIdx + 1
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		i := bytes.IndexByte(line, ':')
		if i == -1 {
			return fmt.Errorf("unmarshal %s: line %d: missing ':'", CacheInfoName, lineno)
		}
		val := bytes.TrimSpace(line[i+1:])
		switch string(line[:i]) {
		case "StoreDir":
			info.StoreDirectory = StoreDirectory(val)
		case "Priority":
			var err error
			info.Priority, err = strconv.Atoi(string(val))
			if err != nil {
				return fmt.Errorf("unmarshal %s: line %d: Priority: %v", CacheInfoName, lineno, err)
			}
		case "WantMassQuery":
			info.WantMassQuery = len(val) == 1 && val[0] == '1'
		}
	}
	return nil
}
