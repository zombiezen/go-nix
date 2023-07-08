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
//
// The caller is responsible for writing files in lexicographical order
// and calling Close at the end to finish the stream.
type Writer struct {
	w   io.Writer
	err error
	// buf is a temporary buffer used for writing.
	// Its length is a multiple of stringAlign
	// that is sufficient to hold any of the known tokens in the NAR format.
	// It is larger than Reader's buffer because we use it to write names.
	buf [64]byte

	state     int8
	hasRoot   bool
	padding   int8
	remaining int64
	lastPath  string
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
		if err := nw.node(hdr); err != nil {
			return err
		}
		if hdr.Path == "" && hdr.Mode.Type() == fs.ModeSymlink {
			nw.state = writerStateEnd
			return nil
		}
	case writerStateFile:
	case writerStateSpecial:
	case writerStateEnd:
	default:
		panic("unreachable")
	}
	return nil
}

func (nw *Writer) node(hdr *Header) error {

	switch hdr.Mode.Type() {
	case 0: // regular
		if hdr.Size < 0 {
			return fmt.Errorf("nar: %s: negative size", hdr.Path)
		}
		nw.write("(")
		nw.write(typeToken)
		nw.write(typeRegular)
		if hdr.Mode&0o111 != 0 {
			nw.write(executableToken)
			nw.write("")
		}
		nw.write(contentsToken)
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
		nw.write(hdr.Linkname)
		nw.write(")")
		nw.state = writerStateSpecial
	default:
		return fmt.Errorf("nar: %s: cannot support mode %v", hdr.Path, hdr.Mode)
	}
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
	return 0, errors.New("TODO(now)")
}

// Close closes the NAR archive by writing the footer.
// If the current file (from a prior call to [Writer.WriteHeader])
// is not fully written, then Close returns an error.
func (nw *Writer) Close() error {
	return errors.New("TODO(now)")
}

func (nw *Writer) writeInt(x uint64) {
	if nw.err != nil {
		return
	}
	binary.LittleEndian.PutUint64(nw.buf[:8], x)
	_, err := nw.Write(nw.buf[:8])
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

// treeDelta computes the directory ends (pops) and/or new directories to be created
// in order to advance from one path to another.
func treeDelta(oldPath string, oldIsDir bool, newPath string) (pop int, newDirs string, err error) {
	oldParent, oldName := slashpath.Split(oldPath)
	newParent, newName := slashpath.Split(newPath)
	if newParent == oldParent {
		if newName <= oldName {
			return 0, "", fmt.Errorf("%s is not ordered after %s", newName, oldName)
		}
		return 0, "", nil
	}
	shared := oldParent
	for ; !strings.HasPrefix(newParent, shared); pop++ {
		shared, _ = slashpath.Split(shared)
	}
	newDirs = newParent[len(shared):]
	// TODO(now): More ordering checks
	return pop, newDirs, nil
}

func validatePath(path string) error {
	if path == "" {
		return nil
	}
	for {
		elemEnd := strings.IndexByte(path, '/')
		if elemEnd == -1 {
			elemEnd = len(path)
		}
		if err := validateFilename(path[:elemEnd]); err != nil {
			return err
		}
		if elemEnd == len(path) {
			return nil
		}
	}
}
