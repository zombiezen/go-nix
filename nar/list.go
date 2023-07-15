package nar

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	slashpath "path"
	"sort"
	"strconv"
	"strings"
)

// Listing is the parsed representation of a ".ls" file,
// an index of a NAR file.
type Listing struct {
	Root ListingNode
}

// List indexes a NAR file.
func List(r io.Reader) (*Listing, error) {
	nr := NewReader(r)
	ls := new(Listing)
	for {
		hdr, err := nr.Next()
		if err == io.EOF {
			return ls, nil
		}
		if err != nil {
			return ls, fmt.Errorf("index nar: %w", err)
		}

		if hdr.Path == "" {
			ls.Root.Header = *hdr
		} else {
			parent, name := slashpath.Split(hdr.Path)
			parent = strings.TrimSuffix(parent, "/")
			curr := ls.lookup(parent)
			if curr.Entries == nil {
				curr.Entries = make(map[string]*ListingNode)
			}
			curr.Entries[name] = &ListingNode{Header: *hdr}
		}
	}
}

// lookup returns the node for the given path or nil if not found.
// The path is assumed to be an unrooted, slash-separated sequence of path elements,
// like "x/y/z".
// Path should not contain elements that are "." or ".." or the empty string,
// except for the special case that the root of the archive is the empty string.
func (ls *Listing) lookup(path string) *ListingNode {
	curr := &ls.Root
	for path != "" {
		i := strings.IndexByte(path, '/')
		end := i + 1
		if i < 0 {
			i = len(path)
			end = i
		}
		name := path[:i]
		next := curr.Entries[name]
		if next == nil {
			return nil
		}
		curr = next
		path = path[end:]
	}
	return curr
}

// MarshalJSON encodes a listing to JSON.
func (ls *Listing) MarshalJSON() ([]byte, error) {
	var buf []byte
	var err error
	buf = append(buf, `{"version":1,"root":`...)
	buf, err = ls.Root.marshal(buf)
	if err != nil {
		return nil, err
	}
	buf = append(buf, `}`...)
	return buf, nil
}

// UnmarshalJSON decodes a listing from JSON.
func (ls *Listing) UnmarshalJSON(data []byte) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("unmarshal nar listing: %w", err)
	}

	var version int
	if err := json.Unmarshal(object["version"], &version); err != nil {
		return fmt.Errorf("unmarshal nar listing: version: %v", err)
	}
	if version != 1 {
		return fmt.Errorf("unmarshal nar listing: unsupported version %d", version)
	}

	for key := range object {
		if key != "version" && key != "root" {
			return fmt.Errorf("unmarshal nar listing: unknown key %q", key)
		}
	}

	if len(object["root"]) == 0 {
		return fmt.Errorf("unmarshal nar listing: missing root")
	}
	ls.Root = ListingNode{}
	if err := ls.Root.unmarshal("", object["root"]); err != nil {
		return fmt.Errorf("unmarshal nar listing: %v", err)
	}

	return nil
}

// ListingNode is an entry in a [Listing].
type ListingNode struct {
	Header
	Entries map[string]*ListingNode
}

func (node *ListingNode) marshal(dst []byte) ([]byte, error) {
	dst = append(dst, `{"type":"`...)
	switch node.Mode.Type() {
	case 0:
		dst = append(dst, typeRegular...)
		dst = append(dst, `","executable":`...)
		if node.Mode&0o111 != 0 {
			dst = append(dst, "true"...)
		} else {
			dst = append(dst, "false"...)
		}
		dst = append(dst, `,"size":`...)
		dst = strconv.AppendInt(dst, node.Size, 10)
		dst = append(dst, `,"narOffset":`...)
		dst = strconv.AppendInt(dst, node.ContentOffset, 10)
	case fs.ModeDir:
		dst = append(dst, typeDirectory...)
		dst = append(dst, `","entries":{`...)
		names := make([]string, 0, len(node.Entries))
		for name := range node.Entries {
			names = append(names, name)
		}
		sort.Strings(names)
		for i, name := range names {
			if i > 0 {
				dst = append(dst, ',')
			}
			nameJSON, err := json.Marshal(name)
			if err != nil {
				return dst, fmt.Errorf("marshal nar listing: entries: %v", err)
			}
			dst = append(dst, nameJSON...)
			dst = append(dst, ':')
			dst, err = node.Entries[name].marshal(dst)
			if err != nil {
				return nil, err
			}
		}
		dst = append(dst, '}')
	case fs.ModeSymlink:
		dst = append(dst, typeSymlink...)
		dst = append(dst, `","target":`...)
		if node.LinkTarget == "" {
			return dst, fmt.Errorf("marshal nar listing: symlink target empty")
		}
		targetString, err := json.Marshal(node.LinkTarget)
		if err != nil {
			return dst, fmt.Errorf("marshal nar listing: target: %v", err)
		}
		dst = append(dst, targetString...)
	default:
		return dst, fmt.Errorf("marshal nar listing: unknown type %v", node.Mode)
	}
	dst = append(dst, '}')
	return dst, nil
}

func (node *ListingNode) unmarshal(path string, data []byte) error {
	if err := validatePath(path); err != nil {
		return err
	}
	node.Path = path

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}

	typData, ok := object["type"]
	if !ok {
		return fmt.Errorf("missing type")
	}
	var typ string
	if err := json.Unmarshal(typData, &typ); err != nil {
		return fmt.Errorf("type: %v", err)
	}
	switch typ {
	case typeRegular:
		node.Mode = modeRegular
	case typeDirectory:
		node.Mode = modeDirectory
	case typeSymlink:
		node.Mode = modeSymlink
	default:
		return fmt.Errorf("type: unknown type %q", typ)
	}

	for k, v := range object {
		switch k {
		case "type":
			// Already parsed.
		case "size":
			if node.Mode.Type() != 0 {
				return fmt.Errorf("size set on %s", typ)
			}
			if err := json.Unmarshal(v, &node.Size); err != nil {
				return fmt.Errorf("size: %v", err)
			}
			if node.Size < 0 {
				return fmt.Errorf("negative size")
			}
		case "target":
			if node.Mode.Type() != fs.ModeSymlink {
				return fmt.Errorf("target set on %s", typ)
			}
			if err := json.Unmarshal(v, &node.LinkTarget); err != nil {
				return fmt.Errorf("target: %v", err)
			}
		case "executable":
			if node.Mode.Type() != 0 {
				return fmt.Errorf("executable set on %s", typ)
			}
			var exec bool
			if err := json.Unmarshal(v, &exec); err != nil {
				return fmt.Errorf("executable: %v", err)
			}
			if exec {
				node.Mode = modeExecutable
			} else {
				node.Mode = modeRegular
			}
		case "narOffset":
			if node.Mode.Type() != 0 {
				return fmt.Errorf("narOffset set on %s", typ)
			}
			if err := json.Unmarshal(v, &node.ContentOffset); err != nil {
				return fmt.Errorf("narOffset: %v", err)
			}
			if node.ContentOffset < 0 {
				return fmt.Errorf("negative content offset")
			}
		case "entries":
			if node.Mode.Type() != fs.ModeDir {
				return fmt.Errorf("entries set on %s", typ)
			}
			var rawEntries map[string]json.RawMessage
			if err := json.Unmarshal(v, &rawEntries); err != nil {
				return fmt.Errorf("entries: %v", err)
			}
			node.Entries = make(map[string]*ListingNode, len(rawEntries))
			for entryName, rawNode := range rawEntries {
				if err := validateFilename(entryName); err != nil {
					return fmt.Errorf("entries: %v", err)
				}
				newNode := new(ListingNode)
				var newPath string
				if path == "" {
					newPath = entryName
				} else {
					newPath = path + "/" + entryName
				}
				if err := newNode.unmarshal(newPath, rawNode); err != nil {
					// TODO(someday): Include path.
					return err
				}
				node.Entries[entryName] = newNode
			}
		default:
			return fmt.Errorf("unknown field %q", k)
		}
	}

	if node.Mode.Type() == fs.ModeSymlink && node.LinkTarget == "" {
		return fmt.Errorf("symlink target not set")
	}
	return nil
}
