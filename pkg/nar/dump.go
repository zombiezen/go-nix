package nar

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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
	nw := NewWriter(w)
	parent := filepath.Dir(path)
	d := &dumper{
		nw:         nw,
		filterFunc: filter,
		fsys:       os.DirFS(parent),
		fsPathToFilterPath: func(p string) string {
			return filepath.Join(parent, filepath.FromSlash(p))
		},
		readlink: func(p string) (string, error) {
			return os.Readlink(filepath.Join(parent, filepath.FromSlash(p)))
		},
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("dump nar: %w", err)
	}
	if !info.IsDir() {
		err := d.dump("", filepath.Base(path), fs.FileInfoToDirEntry(info))
		if err == fs.SkipDir {
			return fmt.Errorf("dump nar: entire path is excluded")
		}
		if err != nil {
			return fmt.Errorf("dump nar: %w", err)
		}
	} else {
		if err := d.recursive(filepath.Base(path)); err != nil {
			return fmt.Errorf("dump nar: %w", err)
		}
	}
	if err := nw.Close(); err != nil {
		return fmt.Errorf("dump nar: %w", err)
	}
	return nil
}

type dumper struct {
	nw                 *Writer
	fsys               fs.FS
	filterFunc         SourceFilterFunc
	readlink           func(string) (string, error)
	fsPathToFilterPath func(string) string
}

func (d *dumper) recursive(rootPath string) error {
	return fs.WalkDir(d.fsys, rootPath, func(path string, ent fs.DirEntry, err error) error {
		var outPath string
		switch {
		case path == rootPath:
			outPath = ""
		case rootPath == ".":
			outPath = path
		default:
			outPath = path[len(rootPath)+len("/"):]
		}
		return d.dump(outPath, path, ent)
	})
}

func (d *dumper) filter(fsPath string, mode fs.FileMode) bool {
	if d.filterFunc == nil {
		return true
	}
	filterPath := fsPath
	if d.fsPathToFilterPath != nil {
		filterPath = d.fsPathToFilterPath(fsPath)
	}
	return d.filterFunc(filterPath, mode)
}

func (d *dumper) dump(outPath string, fsPath string, ent fs.DirEntry) error {
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
		if !d.filter(fsPath, mode) {
			return nil
		}

		err = d.nw.WriteHeader(&Header{
			Path: outPath,
			Mode: mode,
			Size: info.Size(),
		})
		if err != nil {
			return err
		}
		f, err := d.fsys.Open(fsPath)
		if err != nil {
			return err
		}
		_, err = io.Copy(d.nw, f)
		f.Close()
		if err != nil {
			return err
		}
	case fs.ModeDir:
		if !d.filter(fsPath, fs.ModeDir|0o555) {
			return fs.SkipDir
		}
		err := d.nw.WriteHeader(&Header{
			Path: outPath,
			Mode: fs.ModeDir,
		})
		if err != nil {
			return err
		}
	case fs.ModeSymlink:
		if !d.filter(fsPath, fs.ModeSymlink|0o777) {
			return nil
		}
		if d.readlink == nil {
			return fmt.Errorf("cannot process symlink %q on given filesystem", outPath)
		}
		target, err := d.readlink(fsPath)
		if err != nil {
			return err
		}
		err = d.nw.WriteHeader(&Header{
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
