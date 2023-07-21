package nar

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	slashpath "path"
	"path/filepath"
	"sort"
)

// SourceFilterFunc is the interface for creating source filters.
// If the function returns true, the file is copied to the Nix store, otherwise it is omitted.
// This mimics the behaviour of the Nix function [builtins.filterSource].
//
// [builtins.filterSource]: https://nixos.org/manual/nix/stable/language/builtins.html#builtins-filterSource
type SourceFilterFunc func(path string, mode fs.FileMode) bool

// DumpPath will serialize a path on the local file system to NAR format,
// and write it to the passed writer.
func DumpPath(w io.Writer, path string) error {
	return DumpPathFilter(w, path, nil)
}

// DumpPathFilter will serialize a path on the local file system to NAR format,
// and write it to the passed writer, filtering out any files where the filter
// function returns false.
func DumpPathFilter(w io.Writer, path string, filter SourceFilterFunc) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("dump nar: %w", err)
	}
	parent := filepath.Dir(path)
	return dump(filepath.Base(path), fs.FileInfoToDirEntry(info), &dumpOptions{
		nw:         NewWriter(w),
		filterFunc: filter,
		fsys:       os.DirFS(parent),
		fsPathToFilterPath: func(p string) string {
			return filepath.Join(parent, filepath.FromSlash(p))
		},
		readlink: func(p string) (string, error) {
			return os.Readlink(filepath.Join(parent, filepath.FromSlash(p)))
		},
	})
}

// A Dumper contains options for creating a NAR from a filesystem object.
type Dumper struct {
	// FilterFunc filters out files if not nil.
	FilterFunc SourceFilterFunc
	// ReadLink returns the link target of the given path of the filesystem.
	ReadLink func(string) (string, error)
}

// Dump serializes an object in the given filesystem to NAR format,
// writing it to the given writer.
func (d *Dumper) Dump(w io.Writer, fsys fs.FS, path string) error {
	rootEntry, err := lstatFS(fsys, path)
	if err != nil {
		return fmt.Errorf("dump nar: %w", err)
	}
	return dump(path, rootEntry, &dumpOptions{
		nw:         NewWriter(w),
		filterFunc: d.FilterFunc,
		fsys:       fsys,
		readlink:   d.ReadLink,
	})
}

type dumpOptions struct {
	nw                 *Writer
	fsys               fs.FS
	filterFunc         SourceFilterFunc
	readlink           func(string) (string, error)
	fsPathToFilterPath func(string) string
}

func (d *dumpOptions) filter(fsPath string, mode fs.FileMode) bool {
	if d.filterFunc == nil {
		return true
	}
	filterPath := fsPath
	if d.fsPathToFilterPath != nil {
		filterPath = d.fsPathToFilterPath(fsPath)
	}
	return d.filterFunc(filterPath, mode)
}

func dump(path string, lstatEntry fs.DirEntry, opts *dumpOptions) error {
	if !lstatEntry.IsDir() {
		err := dumpSingle("", path, lstatEntry, opts)
		if err == fs.SkipDir {
			return fmt.Errorf("dump nar: entire path is excluded")
		}
		if err != nil {
			return fmt.Errorf("dump nar: %w", err)
		}
	} else {
		if err := dumpRecursive(path, opts); err != nil {
			return fmt.Errorf("dump nar: %w", err)
		}
	}
	if err := opts.nw.Close(); err != nil {
		return fmt.Errorf("dump nar: %w", err)
	}
	return nil
}

func dumpRecursive(rootPath string, opts *dumpOptions) error {
	return fs.WalkDir(opts.fsys, rootPath, func(path string, ent fs.DirEntry, err error) error {
		var outPath string
		switch {
		case path == rootPath:
			outPath = ""
		case rootPath == ".":
			outPath = path
		default:
			outPath = path[len(rootPath)+len("/"):]
		}
		return dumpSingle(outPath, path, ent, opts)
	})
}

func dumpSingle(outPath string, fsPath string, ent fs.DirEntry, opts *dumpOptions) error {
	switch ent.Type() {
	case 0:
		info, err := ent.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()
		if mode.Type() != 0 {
			return fmt.Errorf("%s changed mode from listing=%v to stat=%v", fsPath, ent.Type(), mode)
		}
		if !opts.filter(fsPath, mode) {
			return nil
		}

		err = opts.nw.WriteHeader(&Header{
			Path: outPath,
			Mode: mode,
			Size: info.Size(),
		})
		if err != nil {
			return err
		}
		f, err := opts.fsys.Open(fsPath)
		if err != nil {
			return err
		}
		_, err = io.Copy(opts.nw, f)
		f.Close()
		if err != nil {
			return err
		}
	case fs.ModeDir:
		if !opts.filter(fsPath, fs.ModeDir|0o555) {
			return fs.SkipDir
		}
		err := opts.nw.WriteHeader(&Header{
			Path: outPath,
			Mode: fs.ModeDir,
		})
		if err != nil {
			return err
		}
	case fs.ModeSymlink:
		if !opts.filter(fsPath, fs.ModeSymlink|0o777) {
			return nil
		}
		if opts.readlink == nil {
			return fmt.Errorf("cannot process symlink %q on given filesystem", outPath)
		}
		target, err := opts.readlink(fsPath)
		if err != nil {
			return err
		}
		err = opts.nw.WriteHeader(&Header{
			Path:       outPath,
			Mode:       fs.ModeSymlink,
			LinkTarget: target,
		})
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown type %v for file %v", ent.Type(), fsPath)
	}
	return nil
}

func lstatFS(fsys fs.FS, name string) (fs.DirEntry, error) {
	if name == "." {
		info, err := fs.Stat(fsys, ".")
		if err != nil {
			return nil, err
		}
		return fs.FileInfoToDirEntry(info), nil
	}
	parent := slashpath.Dir(name)
	entries, err := fs.ReadDir(fsys, parent)
	if err != nil {
		return nil, &fs.PathError{
			Op:   "lstat",
			Path: name,
			Err:  err,
		}
	}
	i := sort.Search(len(entries), func(i int) bool {
		return entries[i].Name() >= name
	})
	if i >= len(entries) || entries[i].Name() != name {
		return nil, &fs.PathError{
			Op:   "lstat",
			Path: name,
			Err:  fs.ErrNotExist,
		}
	}
	return entries[i], nil
}
