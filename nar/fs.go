package nar

import (
	"fmt"
	"io"
	"io/fs"
	slashpath "path"
	"sort"
	"strings"
)

// FS implements [fs.FS] for a NAR file.
type FS struct {
	r  io.ReaderAt
	ls *Listing
}

// NewFS returns a new [FS] from a NAR listing
// and a random access reader to the NAR file.
// NewFS will return an error if the listing does not have a directory at its root.
// The listing should not be modified while the returned FS is in use.
func NewFS(r io.ReaderAt, ls *Listing) (*FS, error) {
	if !ls.Root.Mode.IsDir() {
		return nil, fmt.Errorf("new nar fs: not a directory")
	}
	return &FS{r, ls}, nil
}

// Open opens the named file.
func (fsys *FS) Open(name string) (fs.File, error) {
	inode, err := fsys.find(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	if inode.Mode.IsDir() {
		return newFSDir(inode), nil
	}
	return &fsFile{
		inode: inode,
		r:     io.NewSectionReader(fsys.r, inode.ContentOffset, inode.Size),
	}, nil
}

// ReadDir reads the named directory
// and returns a list of directory entries sorted by filename.
func (fsys *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	inode, err := fsys.find(name)
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}
	if !inode.Mode.IsDir() {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fmt.Errorf("not a directory")}
	}
	return newFSDir(inode).entries, nil
}

// Stat returns a [fs.FileInfo] describing the file.
func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	inode, err := fsys.find(name)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}
	return inode.FileInfo(), nil
}

func (fsys *FS) find(path string) (*ListingNode, error) {
	if !fs.ValidPath(path) {
		return nil, fs.ErrInvalid
	}
	if path == "." {
		return &fsys.ls.Root, nil
	}
	curr := &fsys.ls.Root

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
			return nil, fs.ErrNotExist
		}

		if next.Mode.Type() == fs.ModeSymlink {
			if slashpath.IsAbs(next.LinkTarget) {
				return nil, fmt.Errorf("cannot resolve symlink to %s", next.LinkTarget)
			}
			parent := curr.Path
			if parent == "" {
				parent = "."
			}
			// TODO(soon): Prevent cycles.
			var err error
			next, err = fsys.find(slashpath.Join(parent, next.LinkTarget))
			if err != nil {
				return nil, err
			}
		}
		curr = next
		path = path[end:]
	}
	return curr, nil
}

// ReadLink returns the destination of the named symbolic link.
func (fsys *FS) ReadLink(name string) (string, error) {
	if name == "." || !fs.ValidPath(name) {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}
	parent, base := slashpath.Split(name)
	parentNode := &fsys.ls.Root
	if parent != "" {
		var err error
		parentNode, err = fsys.find(parent)
		if err != nil {
			return "", &fs.PathError{Op: "readlink", Path: name, Err: err}
		}
	}
	inode := parentNode.Entries[base]
	if inode == nil {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
	}
	if inode.Mode.Type() != fs.ModeSymlink {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fmt.Errorf("not a symlink")}
	}
	return inode.LinkTarget, nil
}

type fsFile struct {
	inode *ListingNode
	r     *io.SectionReader
}

func (f *fsFile) Stat() (fs.FileInfo, error) {
	return f.inode.FileInfo(), nil
}

func (f *fsFile) Read(p []byte) (int, error) {
	return f.r.Read(p)
}

func (f *fsFile) Seek(offset int64, whence int) (int64, error) {
	return f.r.Seek(offset, whence)
}

func (f *fsFile) ReadAt(p []byte, off int64) (int, error) {
	return f.r.ReadAt(p, off)
}

func (f *fsFile) Close() error {
	return nil
}

type fsDir struct {
	inode   *ListingNode
	entries []fs.DirEntry
}

func newFSDir(inode *ListingNode) *fsDir {
	f := &fsDir{
		inode:   inode,
		entries: make([]fs.DirEntry, 0, len(inode.Entries)),
	}
	for _, child := range inode.Entries {
		f.entries = append(f.entries, fs.FileInfoToDirEntry(child.FileInfo()))
	}
	sort.Slice(f.entries, func(i, j int) bool {
		return f.entries[i].Name() < f.entries[j].Name()
	})
	return f
}

func (f *fsDir) Stat() (fs.FileInfo, error) {
	return f.inode.FileInfo(), nil
}

func (f *fsDir) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("read: is a directory")
}

func (f *fsDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		entries := f.entries
		f.entries = nil
		return entries, nil
	}
	if len(f.entries) == 0 {
		return nil, io.EOF
	}
	entries := f.entries
	if n < len(entries) {
		entries = entries[:n:n]
		f.entries = f.entries[n:]
	} else {
		f.entries = nil
	}
	return entries, nil
}

func (f *fsDir) Close() error {
	return nil
}
