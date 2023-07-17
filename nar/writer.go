package nar

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	slashpath "path"
	"strings"
)

// ErrWriteTooLong is the error returned by [Writer.Write]
// when more bytes are written tha declared in a file's Header.Size.
var ErrWriteTooLong = errors.New("nar: write too long")

const (
	writerStateInit int8 = iota
	writerStateRoot
	writerStateFile
	writerStateSpecial
	writerStateEnd
)

// Writer provides sequential writing of a NAR archive.
// [Writer.WriteHeader] begins a new file with the provided [Header],
// and then Writer can be treated as an [io.Writer] to supply that file's data.
type Writer struct {
	bw bufWriter

	state int8
	// lastPathDir is true if the path named by lastPath is a directory.
	lastPathDir bool
	// remaining is the number of bytes left in a regular file.
	remaining int64
	// lastPath is the path of the last file system object written to the archive.
	lastPath string
}

// NewWriter returns a new [Writer] writing to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{bw: bufWriter{w: w}}
}

// WriteHeader writes hdr and prepares to accept the file's contents.
// The Header.Size field determines how many bytes can be written for the next file.
// If the current file is not fully written, then WriteHeader returns an error.
// Any parent directories named in Header.Path that haven't been written yet
// will automatically be written.
//
// If WriteHeader is called with a Header.Path that is
// equal to or ordered lexicographically before the paths of previous calls to WriteHeader,
// then WriteHeader will return an error.
func (nw *Writer) WriteHeader(hdr *Header) (err error) {
	if nw.bw.err != nil {
		return nw.bw.err
	}
	if err := validatePath(hdr.Path); err != nil {
		return fmt.Errorf("nar: %w", err)
	}

	switch nw.state {
	case writerStateInit:
		nw.bw.string(magic)
		nw.state = writerStateRoot
		fallthrough
	case writerStateRoot:
		if hdr.Path != "" {
			nw.bw.string("(")
			nw.bw.string(typeToken)
			nw.bw.string(typeDirectory)
			nw.lastPath = ""
			nw.lastPathDir = true
		}

		if err := nw.node(hdr); err != nil {
			return err
		}
		if hdr.Path == "" && hdr.Mode.Type() == fs.ModeSymlink {
			nw.state = writerStateEnd
			return nil
		}
	case writerStateFile:
		if nw.lastPath == "" {
			return fmt.Errorf("nar: archive root is a file")
		}
		if nw.remaining > 0 {
			return fmt.Errorf("nar: %d bytes remaining on %s", nw.remaining, formatLastPath(nw.lastPath))
		}
		if err := nw.finishFile(); err != nil {
			return err
		}
		nw.bw.string(")") // finish directory entry

		if err := nw.node(hdr); err != nil {
			return err
		}
	case writerStateSpecial:
		if err := nw.node(hdr); err != nil {
			return err
		}
	case writerStateEnd:
		return fmt.Errorf("nar: root file system object already written")
	default:
		panic("unreachable")
	}
	return nil
}

func (nw *Writer) node(hdr *Header) error {
	if hdr.Mode.IsRegular() && hdr.Size < 0 {
		return fmt.Errorf("nar: %s: negative size", hdr.Path)
	}

	pop, newDirs, err := treeDelta(nw.lastPath, nw.lastPathDir, hdr.Path)
	if err != nil {
		return err
	}
	for i := 0; i < pop; i++ {
		nw.bw.string(")") // directory
		nw.bw.string(")") // parent's entry
	}
	for newDirs != "" {
		name := firstPathComponent(newDirs)
		nw.bw.string(entryToken)
		nw.bw.string("(")
		nw.bw.string(nameToken)
		nw.bw.string(name)
		nw.bw.string(nodeToken)
		nw.bw.string("(")
		nw.bw.string(typeToken)
		nw.bw.string(typeDirectory)

		newDirs = newDirs[len(name):]
		if len(newDirs) >= len("/") {
			newDirs = newDirs[len("/"):]
		}
	}
	if hdr.Path != "" {
		name := slashpath.Base(hdr.Path)
		nw.bw.string(entryToken)
		nw.bw.string("(")
		nw.bw.string(nameToken)
		nw.bw.string(name)
		nw.bw.string(nodeToken)
	}

	switch hdr.Mode.Type() {
	case 0: // regular
		nw.bw.string("(")
		nw.bw.string(typeToken)
		nw.bw.string(typeRegular)
		if hdr.Mode&0o111 != 0 {
			nw.bw.string(executableToken)
			nw.bw.string("")
		}
		nw.bw.string(contentsToken)
		nw.bw.uint64(uint64(hdr.Size))
		nw.state = writerStateFile
		nw.remaining = hdr.Size
	case fs.ModeDir:
		nw.bw.string("(")
		nw.bw.string(typeToken)
		nw.bw.string(typeDirectory)
		nw.state = writerStateSpecial
	case fs.ModeSymlink:
		nw.bw.string("(")
		nw.bw.string(typeToken)
		nw.bw.string(typeSymlink)
		nw.bw.string(targetToken)
		nw.bw.string(hdr.LinkTarget)
		nw.bw.string(")")
		if hdr.Path != "" {
			nw.bw.string(")")
		}
		nw.state = writerStateSpecial
	default:
		return fmt.Errorf("nar: %s: cannot support mode %v", hdr.Path, hdr.Mode)
	}
	nw.bw.flush()
	nw.lastPath = hdr.Path
	nw.lastPathDir = hdr.Mode.IsDir()
	return nw.bw.err
}

// Write writes to the current file in the NAR archive.
// Write returns the error [ErrWriteTooLong]
// if more than Header.Size bytes are written after WriteHeader.
//
// Calling Write on special types like [fs.ModeDir] and [fs.ModeSymlink]
// returns (0, [ErrWriteTooLong]) regardless of what the Header.Size claims.
func (nw *Writer) Write(p []byte) (n int, err error) {
	if nw.state != writerStateFile || nw.remaining <= 0 {
		return 0, ErrWriteTooLong
	}
	if nw.bw.err != nil {
		return 0, nw.bw.err
	}
	tooLong := len(p) > int(nw.remaining)
	if tooLong {
		p = p[:nw.remaining]
	}
	if len(p) > 0 {
		n, err = nw.bw.Write(p)
		if err == nil && tooLong {
			err = ErrWriteTooLong
		}
	}
	nw.remaining -= int64(n)
	return n, err
}

// Offset returns how many bytes have been written to the underlying writer.
// This can be used to determine the "narOffset" of a regular file's contents
// if called immediately after the [Writer.WriteHeader] call
// and before the first call to [Writer.Write].
func (nw *Writer) Offset() int64 {
	return nw.bw.off
}

// Close writes the footer of the NAR archive.
// It does not close the underlying writer.
// If the current file (from a prior call to [Writer.WriteHeader])
// is not fully written, then Close returns an error.
func (nw *Writer) Close() error {
	if nw.bw.err != nil {
		return nw.bw.err
	}
	switch nw.state {
	case writerStateInit, writerStateRoot:
		return fmt.Errorf("nar: close: no object written")
	case writerStateFile:
		if nw.remaining > 0 {
			return fmt.Errorf("nar: close: %d bytes remaining on %s", nw.remaining, formatLastPath(nw.lastPath))
		}
		nw.finishFile()
		if nw.lastPath != "" {
			nw.bw.string(")") // finish directory entry
		}
	case writerStateEnd:
		nw.bw.err = errors.New("nar: writer closed")
		return nil
	}

	pop := strings.Count(nw.lastPath, "/")
	if nw.lastPath != "" && nw.lastPathDir {
		pop++
	}
	for i := 0; i < pop; i++ {
		nw.bw.string(")") // directory
		nw.bw.string(")") // parent's entry
	}
	if nw.lastPath != "" || nw.lastPathDir {
		nw.bw.string(")") // root directory
	}

	nw.bw.flush()
	prevErr := nw.bw.err
	if nw.bw.err == nil {
		nw.bw.err = errors.New("nar: writer closed")
	}
	return prevErr
}

func (nw *Writer) finishFile() error {
	nw.bw.pad()
	nw.bw.string(")")
	return nw.bw.err
}

// bufWriter implements buffered NAR string writing.
type bufWriter struct {
	w io.Writer
	// off is the number of bytes written to w.
	// It does not include bytes written to buf.
	off int64
	// err is the first error returned by w.
	err error
	// buf is a temporary buffer used for writing.
	// Its length is a multiple of stringAlign
	// that is sufficient to hold any of the known tokens in the NAR format.
	buf    [256]byte
	bufLen int16
}

// Write passes through a write to the underlying writer.
func (bw *bufWriter) Write(p []byte) (n int, err error) {
	bw.flush()
	n, err = bw.w.Write(p)
	bw.off += int64(n)
	return n, err
}

// flush writes any buffered data to the underlying writer.
func (bw *bufWriter) flush() {
	if bw.err != nil || bw.bufLen == 0 {
		return
	}
	n, err := bw.w.Write(bw.buf[:bw.bufLen])
	copy(bw.buf[:], bw.buf[n:bw.bufLen])
	bw.bufLen -= int16(n)
	bw.off += int64(n)
	if err != nil {
		bw.err = fmt.Errorf("nar: %w", err)
	}
}

// uint64 writes a little-endian 64-bit integer.
func (bw *bufWriter) uint64(x uint64) {
	if bw.err != nil {
		return
	}
	bw.pad()
	if int(bw.bufLen)+8 > len(bw.buf) {
		bw.flush()
		if bw.err != nil {
			return
		}
	}
	binary.LittleEndian.PutUint64(bw.buf[bw.bufLen:], x)
	bw.bufLen += 8
}

// string writes a string prefixed by its length.
func (bw *bufWriter) string(s string) {
	bw.uint64(uint64(len(s)))
	if len(s) == 0 || bw.err != nil {
		return
	}
	n := padStringSize(len(s))
	if n > len(bw.buf) {
		bw.longString(s)
		return
	}

	if int(bw.bufLen)+n > len(bw.buf) {
		// String *will* fit in buffer once flushed.
		nn := copy(bw.buf[bw.bufLen:], s)
		bw.bufLen = int16(len(bw.buf))
		n -= nn
		s = s[nn:]
		bw.flush()
		if bw.err != nil {
			return
		}
	}

	// String fits in buffer.
	nn := copy(bw.buf[bw.bufLen:], s)
	bw.bufLen += int16(nn)
	bw.pad()
}

func (bw *bufWriter) longString(s string) {
	// Less common case: string/token does not fit in buffer.
	// Try to use WriteString if possible, otherwise multiple Writes.

	if sw, ok := bw.w.(io.StringWriter); ok {
		bw.flush()
		if bw.err != nil {
			return
		}

		n, err := sw.WriteString(s)
		bw.off += int64(n)
		if err != nil {
			bw.err = fmt.Errorf("nar: %w", err)
			return
		}
	} else {
		for i := 0; i < len(s); {
			if int(bw.bufLen) >= len(bw.buf) {
				bw.flush()
				if bw.err != nil {
					return
				}
			}
			n := copy(bw.buf[bw.bufLen:], s[i:])
			bw.bufLen += int16(n)
			i += n
		}
	}

	bw.pad()
}

// pad writes zero bytes until bw.off+bw.bufLen is evenly divisible by stringAlign.
func (bw *bufWriter) pad() {
	if bw.err != nil {
		return
	}
	n := stringPaddingLength(int(bw.off%stringAlign) + int(bw.bufLen))
	if int(bw.bufLen)+n > len(bw.buf) {
		n -= len(bw.buf) - int(bw.bufLen)
		for ; int(bw.bufLen) < len(bw.buf); bw.bufLen++ {
			bw.buf[bw.bufLen] = 0
		}
		bw.flush()
		if bw.err != nil {
			return
		}
	}
	for n > 0 {
		bw.buf[bw.bufLen] = 0
		bw.bufLen++
		n--
	}
}

// treeDelta computes the directory ends (pops) and/or new directories to be created
// in order to advance from one path to another.
func treeDelta(oldPath string, oldIsDir bool, newPath string) (pop int, newDirs string, err error) {
	newParent, _ := slashpath.Split(newPath)
	if shared := oldPath + "/"; strings.HasPrefix(newPath, shared) {
		if !oldIsDir {
			return 0, "", fmt.Errorf("%s is not a directory", formatLastPath(oldPath))
		}
		newDirs = strings.TrimSuffix(newParent[len(shared):], "/")
		return pop, strings.TrimSuffix(newDirs, "/"), nil
	}

	var shared string
	switch {
	case oldIsDir && oldPath == "":
		shared = oldPath
	case oldIsDir && oldPath != "":
		shared = oldPath + "/"
	default:
		shared, _ = slashpath.Split(oldPath)
	}
	for ; !strings.HasPrefix(newParent, shared); pop++ {
		shared, _ = slashpath.Split(strings.TrimSuffix(shared, "/"))
	}

	if oldPath != "" && newPath != "" {
		newName := firstPathComponent(newPath[len(shared):])
		oldName := firstPathComponent(oldPath[len(shared):])
		if newName <= oldName {
			return 0, "", fmt.Errorf("%s is not ordered after %s",
				formatLastPath(newPath[:len(shared)+len(newName)]),
				formatLastPath(oldPath[:len(shared)+len(oldName)]))
		}
	}

	newDirs = strings.TrimSuffix(newParent[len(shared):], "/")
	return pop, newDirs, nil
}

func firstPathComponent(path string) string {
	i := strings.IndexByte(path, '/')
	if i == -1 {
		return path
	}
	return path[:i]
}

func formatLastPath(s string) string {
	if s == "" {
		return "<root filesystem object>"
	}
	return s
}

func validatePath(origPath string) error {
	if origPath == "" {
		return nil
	}
	path := origPath
	for {
		elemEnd := strings.IndexByte(path, '/')
		if elemEnd == -1 {
			elemEnd = len(path)
		}
		if path[:elemEnd] == "" {
			return fmt.Errorf("%q has empty elements", origPath)
		}
		if err := validateFilename(path[:elemEnd]); err != nil {
			return err
		}
		if elemEnd == len(path) {
			return nil
		}
		path = path[elemEnd+1:]
	}
}
