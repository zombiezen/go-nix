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
	w   io.Writer
	err error
	// buf is a temporary buffer used for writing.
	// Its length is a multiple of stringAlign
	// that is sufficient to hold any of the known tokens in the NAR format.
	// It is larger than Reader's buffer because we use it to write names.
	buf [64]byte

	state       int8
	lastPathDir bool
	padding     int8
	remaining   int64
	lastPath    string
}

// NewWriter returns a new [Writer] writing to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
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
	if nw.err != nil {
		return nw.err
	}
	if err := validatePath(hdr.Path); err != nil {
		return fmt.Errorf("nar: %w", err)
	}

	switch nw.state {
	case writerStateInit:
		nw.write(magic)
		nw.state = writerStateRoot
		fallthrough
	case writerStateRoot:
		if hdr.Path != "" {
			nw.write("(")
			nw.write(typeToken)
			nw.write(typeDirectory)
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
		nw.write(")") // finish directory entry

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
		nw.write(")") // directory
		nw.write(")") // parent's entry
	}
	for newDirs != "" {
		name := firstPathComponent(newDirs)
		nw.write(entryToken)
		nw.write("(")
		nw.write(nameToken)
		nw.write(name)
		nw.write(nodeToken)
		nw.write("(")
		nw.write(typeToken)
		nw.write(typeDirectory)

		newDirs = newDirs[len(name):]
		if len(newDirs) >= len("/") {
			newDirs = newDirs[len("/"):]
		}
	}
	if hdr.Path != "" {
		name := slashpath.Base(hdr.Path)
		nw.write(entryToken)
		nw.write("(")
		nw.write(nameToken)
		nw.write(name)
		nw.write(nodeToken)
	}

	switch hdr.Mode.Type() {
	case 0: // regular
		nw.write("(")
		nw.write(typeToken)
		nw.write(typeRegular)
		if hdr.Mode&0o111 != 0 {
			nw.write(executableToken)
			nw.write("")
		}
		nw.write(contentsToken)
		nw.writeInt(uint64(hdr.Size))
		nw.state = writerStateFile
		nw.remaining = hdr.Size
		nw.padding = int8(stringPaddingLength(int(hdr.Size % stringAlign)))
	case fs.ModeDir:
		nw.write("(")
		nw.write(typeToken)
		nw.write(typeDirectory)
		nw.state = writerStateSpecial
	case fs.ModeSymlink:
		nw.write("(")
		nw.write(typeToken)
		nw.write(typeSymlink)
		nw.write(targetToken)
		nw.write(hdr.LinkTarget)
		nw.write(")")
		if hdr.Path != "" {
			nw.write(")")
		}
		nw.state = writerStateSpecial
	default:
		return fmt.Errorf("nar: %s: cannot support mode %v", hdr.Path, hdr.Mode)
	}
	nw.lastPath = hdr.Path
	nw.lastPathDir = hdr.Mode.IsDir()
	return nw.err
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
	if nw.err != nil {
		return 0, nw.err
	}
	tooLong := len(p) > int(nw.remaining)
	if tooLong {
		p = p[:nw.remaining]
	}
	if len(p) > 0 {
		n, err = nw.w.Write(p)
		if err == nil && tooLong {
			err = ErrWriteTooLong
		}
	}
	nw.remaining -= int64(n)
	return n, err
}

// Close writes the footer of the NAR archive.
// It does not close the underlying writer.
// If the current file (from a prior call to [Writer.WriteHeader])
// is not fully written, then Close returns an error.
func (nw *Writer) Close() error {
	if nw.err != nil {
		return nw.err
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
			nw.write(")") // finish directory entry
		}
	case writerStateEnd:
		nw.err = errors.New("nar: writer closed")
		return nil
	}

	pop := strings.Count(nw.lastPath, "/")
	if nw.lastPath != "" {
		pop++
	}
	if nw.lastPathDir {
		pop++
	}
	for i := 0; i < pop; i++ {
		nw.write(")")
	}

	prevErr := nw.err
	if nw.err == nil {
		nw.err = errors.New("nar: writer closed")
	}
	return prevErr
}

func (nw *Writer) writeInt(x uint64) {
	if nw.err != nil {
		return
	}
	binary.LittleEndian.PutUint64(nw.buf[:8], x)
	_, err := nw.w.Write(nw.buf[:8])
	if err != nil {
		nw.err = fmt.Errorf("nar: %w", err)
	}
}

func (nw *Writer) write(s string) {
	nw.writeInt(uint64(len(s)))
	if len(s) == 0 || nw.err != nil {
		return
	}

	if len(s) > len(nw.buf) {
		// Less common case: string/token does not fit in buffer.
		// Try to use WriteString if possible, otherwise multiple Writes.

		if sw, ok := nw.w.(io.StringWriter); ok {
			if _, err := sw.WriteString(s); err != nil {
				nw.err = fmt.Errorf("nar: %w", err)
				return
			}
		} else {
			for i := 0; i < len(s); {
				n := copy(nw.buf[:], s[i:])
				if _, err := nw.w.Write(nw.buf[:n]); err != nil {
					nw.err = fmt.Errorf("nar: %w", err)
					return
				}
				i += n
			}
		}

		if padding := stringPaddingLength(len(s)); padding > 0 {
			for i := 0; i < padding; i++ {
				nw.buf[i] = 0
			}
			if _, err := nw.w.Write(nw.buf[:padding]); err != nil {
				nw.err = fmt.Errorf("nar: %w", err)
			}
		}
		return
	}

	// Common case: string/token fits in buffer.
	// Write string and padding in single Write.
	copy(nw.buf[:], s)
	n := padStringSize(len(s))
	for i := len(s); i < n; i++ {
		nw.buf[i] = 0
	}
	if _, err := nw.w.Write(nw.buf[:n]); err != nil {
		nw.err = fmt.Errorf("nar: %w", err)
	}
}

func (nw *Writer) finishFile() error {
	if nw.err != nil {
		return nw.err
	}
	if nw.padding > 0 {
		for i := int8(0); i < nw.padding; i++ {
			nw.buf[i] = 0
		}
		if _, err := nw.w.Write(nw.buf[:nw.padding]); err != nil {
			nw.err = fmt.Errorf("nar: %w", err)
		}
	}
	nw.write(")")
	return nw.err
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
