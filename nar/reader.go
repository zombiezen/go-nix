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
	r     io.Reader
	buf   [16]byte
	state int8

	// padding is the number of padding bytes that trail after the file contents
	// (only valid if state == readerStateFile).
	padding int8
	// remaining is the number of bytes remaining in file contents
	// (only valid if state == readerStateFile).
	remaining int64
	// prefix is the current directory's path including a trailing slash.
	prefix string
	// err is the error to return for future calls to Next or Read.
	err error
}

// A Header represents a single header in a NAR archive.
// Some fields may not be populated.
type Header struct {
	// Path is a UTF-8 encoded, unrooted, slash-separated sequence of path elements,
	// like "x/y/z".
	// Path will not contain elements that are "." or ".." or the empty string,
	// except for the special case where an archive consists of a single file
	// will use the empty string.
	Path string
	// Mode is the type of the file system object.
	Mode fs.FileMode
	// Size is the size of a regular file in bytes.
	Size int64
	// LinkTarget is the target of a symlink.
	LinkTarget string
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
	defer func() {
		if err != nil && r.err == nil {
			r.err = errInvalid
		}
	}()

	if r.err != nil {
		return nil, r.err
	}
	switch r.state {
	case readerStateFirst:
		if err := r.expectString(magic); err != nil {
			return nil, fmt.Errorf("nar: magic number: %w", err)
		}
		hdr := new(Header)
		if err := r.node(hdr); err != nil {
			return nil, fmt.Errorf("nar: %w", err)
		}
		if r.state == readerStateFirst {
			// Self-contained first Next call (symlink).
			// Will return error on next call to Next.
			r.verifyEOF()
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
		if err := r.expectString(")"); err != nil {
			return nil, fmt.Errorf("nar: %w", err)
		}

		// Now advance to next header.
		if r.prefix == "" {
			// Only a file. Should be at EOF.
			r.verifyEOF()
			return nil, r.err
		}
		r.state = readerStateDirectory
		fallthrough
	case readerStateDirectory, readerStateDirectoryStart:
		// Close out the entry parenthesis.
		if r.state != readerStateDirectoryStart {
			if err := r.expectString(")"); err != nil {
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
				prevSlash := strings.LastIndexByte(r.prefix[:len(r.prefix)-len("/")], '/')
				r.prefix = r.prefix[:prevSlash+len("/")]
			case entryToken:
				break popLoop
			default:
				return nil, fmt.Errorf("nar: directory: got %q token (expected \")\" or %q)", r.buf[:n], entryToken)
			}
		}

		if err := r.expectString("("); err != nil {
			return nil, fmt.Errorf("nar: directory: %w", err)
		}
		if err := r.expectString(nameToken); err != nil {
			return nil, fmt.Errorf("nar: directory: %w", err)
		}
		name, err := r.readString(entryNameMaxLen)
		if err != nil {
			return nil, fmt.Errorf("nar: directory: entry name: %w", err)
		}
		if err := validateFilename(name); err != nil {
			return nil, fmt.Errorf("nar: directory: entry name: %v", err)
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

func (r *Reader) node(hdr *Header) error {
	if err := r.expectString("("); err != nil {
		return err
	}
	if err := r.expectString("type"); err != nil {
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
			if err := r.expectString(""); err != nil {
				return err
			}
			if err := r.expectString(contentsToken); err != nil {
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
		r.state = readerStateDirectoryStart
	case typeSymlink:
		if err := r.expectString(targetToken); err != nil {
			return fmt.Errorf("symlink: %w", err)
		}
		var err error
		hdr.LinkTarget, err = r.readString(symlinkTargetMaxLen)
		if err != nil {
			return fmt.Errorf("symlink target: %w", err)
		}
		hdr.Mode = fs.ModeSymlink | 0o777
		if err := r.expectString(")"); err != nil {
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

func (r *Reader) expectString(s string) error {
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
