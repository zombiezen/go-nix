package nar

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
)

var (
	errInvalid      = errors.New("nar: invalid data")
	errTrailingData = errors.New("nar: trailing data")
)

const (
	readerStateFirst int8 = iota
	readerStateFile
	readerStateDirectoryStart
	readerStateDirectory
)

// Reader provides sequential access to the contents of a NAR archive.
// [Reader.Next] advances to the next file in the archive (including the first),
// and then Reader can be treated as an [io.Reader] to access the file's data.
type Reader struct {
	r io.Reader
	// buf is a temporary buffer used for reading.
	// Its length is a multiple of stringAlign
	// that is sufficient to hold any of the known tokens in the NAR format.
	buf   [16]byte
	state int8

	// padding is the number of padding bytes that trail after the file contents
	// (only valid if state == readerStateFile).
	padding int8
	// hasRoot is true if the root file system object is a directory.
	hasRoot bool
	// remaining is the number of bytes remaining in file contents
	// (only valid if state == readerStateFile).
	remaining int64
	// prefix is the current directory's path including a trailing slash.
	prefix string
	// err is the error to return for future calls to Next or Read.
	err error
}

// NewReader creates a new [Reader] reading from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

// Next advances to the next entry in the NAR archive.
// The Header.Size determines how many bytes can be read for the next file.
// Any remaining data in the current file is automatically discarded.
// At the end of the archive, Next returns the error [io.EOF].
func (r *Reader) Next() (_ *Header, err error) {
	if r.err != nil {
		return nil, r.err
	}
	defer func() {
		if err != nil && r.err == nil {
			r.err = errInvalid
		}
	}()

	switch r.state {
	case readerStateFirst:
		if err := r.expect(magic); err != nil {
			return nil, fmt.Errorf("nar: magic number: %w", err)
		}
		hdr := new(Header)
		if err := r.node(hdr); err != nil {
			return nil, fmt.Errorf("nar: %w", err)
		}
		switch r.state {
		case readerStateFirst:
			// Self-contained first Next call (symlink).
			// Will return error on next call to Next.
			r.verifyEOF()
		case readerStateDirectoryStart:
			r.hasRoot = true
		}
		return hdr, nil
	case readerStateFile:
		// Advance to end of file.
		_, err := io.CopyN(io.Discard, r.r, r.remaining+int64(r.padding))
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		if err != nil {
			r.err = fmt.Errorf("nar: %w", err)
			return nil, r.err
		}
		if err := r.expect(")"); err != nil {
			return nil, fmt.Errorf("nar: %w", err)
		}

		// Now advance to next header.
		if !r.hasRoot {
			r.verifyEOF()
			return nil, r.err
		}
		r.state = readerStateDirectory
		fallthrough
	case readerStateDirectory, readerStateDirectoryStart:
		// Close out the previous entry's parenthesis.
		if r.state != readerStateDirectoryStart {
			if err := r.expect(")"); err != nil {
				return nil, fmt.Errorf("nar: %w", err)
			}
			r.state = readerStateDirectory
		}

	popLoop:
		for {
			n, err := r.readSmallString()
			if err != nil {
				return nil, fmt.Errorf("nar: %w", err)
			}
			switch string(r.buf[:n]) {
			case ")":
				// Pop up a directory.
				if r.prefix == "" {
					r.verifyEOF()
					return nil, r.err
				}
				// Close out the directory entry's parenthesis.
				if err := r.expect(")"); err != nil {
					return nil, fmt.Errorf("nar: %w", err)
				}
				prevSlash := strings.LastIndexByte(r.prefix[:len(r.prefix)-len("/")], '/')
				if prevSlash < 0 {
					r.prefix = ""
				} else {
					r.prefix = r.prefix[:prevSlash+len("/")]
				}
			case entryToken:
				break popLoop
			default:
				return nil, fmt.Errorf("nar: directory: got %q token (expected \")\" or %q)", r.buf[:n], entryToken)
			}
		}

		if err := r.expect("("); err != nil {
			return nil, fmt.Errorf("nar: directory: %w", err)
		}
		if err := r.expect(nameToken); err != nil {
			return nil, fmt.Errorf("nar: directory: %w", err)
		}
		name, err := r.readString(entryNameMaxLen)
		if err != nil {
			return nil, fmt.Errorf("nar: directory: entry name: %w", err)
		}
		if err := validateFilename(name); err != nil {
			return nil, fmt.Errorf("nar: directory: entry name: %v", err)
		}
		if err := r.expect(nodeToken); err != nil {
			return nil, fmt.Errorf("nar: directory: %w", err)
		}
		hdr := &Header{Path: r.prefix + name}
		if err := r.node(hdr); err != nil {
			return nil, fmt.Errorf("nar: %w", err)
		}
		return hdr, nil
	default:
		panic("unreachable")
	}
}

// Read reads from the current file in the NAR archive.
// It returns (0, io.EOF) when it reaches the end of that file,
// until [Reader.Next] is called to advance to the next file.
//
// Calling Read on special types like [fs.ModeDir] and [fs.ModeSymlink]
// returns (0, io.EOF).
func (r *Reader) Read(p []byte) (n int, err error) {
	if r.state != readerStateFile || r.remaining <= 0 {
		// Special files or EOF always should report io.EOF,
		// even if there are other errors present.
		return 0, io.EOF
	}
	if r.err != nil {
		return 0, r.err
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err = r.r.Read(p)
	r.remaining -= int64(n)
	if err == io.EOF {
		// Files have a closing parenthesis token,
		// so encountering an EOF from the underlying reader is always unexpected.
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		r.err = fmt.Errorf("nar: %w", err)
	}
	if r.remaining <= 0 {
		// If we've hit the regular file's contents boundary,
		// let the Read report success (since we did return all the bytes)
		// but then the next call to Next will fail.
		err = nil
	}
	return n, err
}

func (r *Reader) node(hdr *Header) error {
	if err := r.expect("("); err != nil {
		return err
	}
	if err := r.expect("type"); err != nil {
		return err
	}
	n, err := r.readSmallString()
	if err != nil {
		return fmt.Errorf("type: %w", err)
	}
	switch string(r.buf[:n]) {
	case typeRegular:
		n, err := r.readSmallString()
		if err != nil {
			return fmt.Errorf("regular: %w", err)
		}
		hdr.Mode = 0o444
		switch string(r.buf[:n]) {
		case executableToken:
			hdr.Mode |= 0o111
			if err := r.expect(""); err != nil {
				return err
			}
			if err := r.expect(contentsToken); err != nil {
				return err
			}
		case contentsToken:
			// Do nothing.
		default:
			return fmt.Errorf("regular: got %q token (expected %q or %q)", r.buf[:n], executableToken, contentsToken)
		}
		unsignedSize, err := r.readInt()
		if err != nil {
			return err
		}
		if unsignedSize >= 1<<63 {
			return fmt.Errorf("file too large (%d bytes)", unsignedSize)
		}
		hdr.Size = int64(unsignedSize)
		r.state = readerStateFile
		r.remaining = int64(unsignedSize)
		r.padding = int8(stringPaddingLength(int(unsignedSize % stringAlign)))
	case typeDirectory:
		if hdr.Path != "" {
			r.prefix = hdr.Path + "/"
		}
		hdr.Mode = fs.ModeDir | 0o555
		r.state = readerStateDirectoryStart
	case typeSymlink:
		if err := r.expect(targetToken); err != nil {
			return fmt.Errorf("symlink: %w", err)
		}
		var err error
		hdr.Linkname, err = r.readString(symlinkTargetMaxLen)
		if err != nil {
			return fmt.Errorf("symlink target: %w", err)
		}
		hdr.Mode = fs.ModeSymlink | 0o777
		if err := r.expect(")"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid node type %q", r.buf[:n])
	}
	return nil
}

// verifyEOF consumes a single byte to verify that the reader is at EOF.
// r.err will always be non-nil after verifyEOF returns.
func (r *Reader) verifyEOF() {
	switch _, err := io.ReadFull(r.r, r.buf[:1]); err {
	case nil:
		r.err = errTrailingData
	case io.EOF:
		r.err = io.EOF
	default:
		r.err = fmt.Errorf("nar: at eof: %w", err)
	}
}

func (r *Reader) read(p []byte) error {
	_, err := io.ReadFull(r.r, p)
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		r.err = err
		return err
	}
	return nil
}

func (r *Reader) readInt() (uint64, error) {
	if err := r.read(r.buf[:8]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(r.buf[:8]), nil
}

// readSmallString reads a small string into r.buf,
// returning how many bytes were read.
// Because of the NAR file structure,
// any [io.EOF] is treated as [io.ErrUnexpectedEOF].
func (r *Reader) readSmallString() (n int, err error) {
	nn, err := r.readInt()
	if err != nil {
		return 0, err
	}
	if nn > uint64(len(r.buf)) {
		return 0, fmt.Errorf("got string of length %d (max %d in this context)", nn, len(r.buf))
	}
	if err := r.read(r.buf[:padStringSize(int(nn))]); err != nil {
		return 0, err
	}
	return int(nn), nil
}

func (r *Reader) readString(maxLength int) (string, error) {
	n, err := r.readInt()
	if err != nil {
		return "", err
	}
	if n > uint64(maxLength) {
		return "", fmt.Errorf("got string of length %d (max %d in this context)", n, maxLength)
	}
	buf := make([]byte, padStringSize(int(n)))
	if err := r.read(buf); err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

func (r *Reader) expect(s string) error {
	n, err := r.readSmallString()
	if err != nil {
		return err
	}
	// Under gc compiler, string conversion will not allocate.
	// https://github.com/golang/go/wiki/CompilerOptimizations#conversion-for-string-comparison
	if string(r.buf[:n]) != s {
		return fmt.Errorf("got %q token (expected %q token)", string(r.buf[:n]), s)
	}
	return nil
}
