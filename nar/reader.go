package nar

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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
	r   io.Reader
	off int64
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
func (nr *Reader) Next() (_ *Header, err error) {
	if nr.err != nil {
		return nil, nr.err
	}
	defer func() {
		if err != nil && nr.err == nil {
			nr.err = errInvalid
		}
	}()

	switch nr.state {
	case readerStateFirst:
		if err := nr.expect(magic); err != nil {
			return nil, fmt.Errorf("nar: magic number: %w", err)
		}
		hdr := new(Header)
		if err := nr.node(hdr); err != nil {
			return nil, fmt.Errorf("nar: %w", err)
		}
		switch nr.state {
		case readerStateFirst:
			// Self-contained first Next call (symlink).
			// Will return error on next call to Next.
			nr.verifyEOF()
		case readerStateDirectoryStart:
			nr.hasRoot = true
		}
		return hdr, nil
	case readerStateFile:
		// Advance to end of file.
		n, err := io.CopyN(io.Discard, nr.r, nr.remaining+int64(nr.padding))
		nr.off += n
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		if err != nil {
			nr.err = fmt.Errorf("nar: %w", err)
			return nil, nr.err
		}
		if err := nr.expect(")"); err != nil {
			return nil, fmt.Errorf("nar: %w", err)
		}

		// Now advance to next header.
		if !nr.hasRoot {
			nr.verifyEOF()
			return nil, nr.err
		}
		nr.state = readerStateDirectory
		fallthrough
	case readerStateDirectory, readerStateDirectoryStart:
		// Close out the previous entry's parenthesis.
		if nr.state != readerStateDirectoryStart {
			if err := nr.expect(")"); err != nil {
				return nil, fmt.Errorf("nar: %w", err)
			}
			nr.state = readerStateDirectory
		}

	popLoop:
		for {
			n, err := nr.readSmallString()
			if err != nil {
				return nil, fmt.Errorf("nar: %w", err)
			}
			switch string(nr.buf[:n]) {
			case ")":
				// Pop up a directory.
				if nr.prefix == "" {
					nr.verifyEOF()
					return nil, nr.err
				}
				// Close out the directory entry's parenthesis.
				if err := nr.expect(")"); err != nil {
					return nil, fmt.Errorf("nar: %w", err)
				}
				prevSlash := strings.LastIndexByte(nr.prefix[:len(nr.prefix)-len("/")], '/')
				if prevSlash < 0 {
					nr.prefix = ""
				} else {
					nr.prefix = nr.prefix[:prevSlash+len("/")]
				}
			case entryToken:
				break popLoop
			default:
				return nil, fmt.Errorf("nar: directory: got %q token (expected \")\" or %q)", nr.buf[:n], entryToken)
			}
		}

		if err := nr.expect("("); err != nil {
			return nil, fmt.Errorf("nar: directory: %w", err)
		}
		if err := nr.expect(nameToken); err != nil {
			return nil, fmt.Errorf("nar: directory: %w", err)
		}
		name, err := nr.readString(entryNameMaxLen)
		if err != nil {
			return nil, fmt.Errorf("nar: directory: entry name: %w", err)
		}
		if err := validateFilename(name); err != nil {
			return nil, fmt.Errorf("nar: directory: entry name: %v", err)
		}
		if err := nr.expect(nodeToken); err != nil {
			return nil, fmt.Errorf("nar: directory: %w", err)
		}
		hdr := &Header{Path: nr.prefix + name}
		if err := nr.node(hdr); err != nil {
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
func (nr *Reader) Read(p []byte) (n int, err error) {
	if nr.state != readerStateFile || nr.remaining <= 0 {
		// Special files or EOF always should report io.EOF,
		// even if there are other errors present.
		return 0, io.EOF
	}
	if nr.err != nil {
		return 0, nr.err
	}
	if int64(len(p)) > nr.remaining {
		p = p[:nr.remaining]
	}
	n, err = nr.r.Read(p)
	nr.off += int64(n)
	nr.remaining -= int64(n)
	if err == io.EOF {
		// Files have a closing parenthesis token,
		// so encountering an EOF from the underlying reader is always unexpected.
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		nr.err = fmt.Errorf("nar: %w", err)
	}
	if nr.remaining <= 0 {
		// If we've hit the regular file's contents boundary,
		// let the Read report success (since we did return all the bytes)
		// but then the next call to Next will fail.
		err = nil
	}
	return n, err
}

func (nr *Reader) node(hdr *Header) error {
	if err := nr.expect("("); err != nil {
		return err
	}
	if err := nr.expect("type"); err != nil {
		return err
	}
	n, err := nr.readSmallString()
	if err != nil {
		return fmt.Errorf("type: %w", err)
	}
	switch string(nr.buf[:n]) {
	case typeRegular:
		n, err := nr.readSmallString()
		if err != nil {
			return fmt.Errorf("regular: %w", err)
		}
		hdr.Mode = modeRegular
		switch string(nr.buf[:n]) {
		case executableToken:
			hdr.Mode = modeExecutable
			if err := nr.expect(""); err != nil {
				return err
			}
			if err := nr.expect(contentsToken); err != nil {
				return err
			}
		case contentsToken:
			// Do nothing.
		default:
			return fmt.Errorf("regular: got %q token (expected %q or %q)", nr.buf[:n], executableToken, contentsToken)
		}
		unsignedSize, err := nr.readInt()
		if err != nil {
			return err
		}
		if unsignedSize >= 1<<63 {
			return fmt.Errorf("file too large (%d bytes)", unsignedSize)
		}
		hdr.Size = int64(unsignedSize)
		hdr.ContentOffset = nr.off
		nr.state = readerStateFile
		nr.remaining = int64(unsignedSize)
		nr.padding = int8(stringPaddingLength(int(unsignedSize % stringAlign)))
	case typeDirectory:
		if hdr.Path != "" {
			nr.prefix = hdr.Path + "/"
		}
		hdr.Mode = modeDirectory
		nr.state = readerStateDirectoryStart
	case typeSymlink:
		if err := nr.expect(targetToken); err != nil {
			return fmt.Errorf("symlink: %w", err)
		}
		var err error
		hdr.LinkTarget, err = nr.readString(symlinkTargetMaxLen)
		if err != nil {
			return fmt.Errorf("symlink target: %w", err)
		}
		hdr.Mode = modeSymlink
		if err := nr.expect(")"); err != nil {
			return err
		}
		if nr.state == readerStateDirectoryStart {
			nr.state = readerStateDirectory
		}
	default:
		return fmt.Errorf("invalid node type %q", nr.buf[:n])
	}
	return nil
}

// verifyEOF consumes a single byte to verify that the reader is at EOF.
// r.err will always be non-nil after verifyEOF returns.
func (nr *Reader) verifyEOF() {
	switch _, err := io.ReadFull(nr.r, nr.buf[:1]); err {
	case nil:
		nr.off++
		nr.err = errTrailingData
	case io.EOF:
		nr.err = io.EOF
	default:
		nr.err = fmt.Errorf("nar: at eof: %w", err)
	}
}

func (nr *Reader) read(p []byte) error {
	n, err := io.ReadFull(nr.r, p)
	nr.off += int64(n)
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		nr.err = err
		return err
	}
	return nil
}

func (nr *Reader) readInt() (uint64, error) {
	if err := nr.read(nr.buf[:8]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(nr.buf[:8]), nil
}

// readSmallString reads a small string into r.buf,
// returning how many bytes were read.
// Because of the NAR file structure,
// any [io.EOF] is treated as [io.ErrUnexpectedEOF].
func (nr *Reader) readSmallString() (n int, err error) {
	nn, err := nr.readInt()
	if err != nil {
		return 0, err
	}
	if nn > uint64(len(nr.buf)) {
		return 0, fmt.Errorf("got string of length %d (max %d in this context)", nn, len(nr.buf))
	}
	if err := nr.read(nr.buf[:padStringSize(int(nn))]); err != nil {
		return 0, err
	}
	return int(nn), nil
}

func (nr *Reader) readString(maxLength int) (string, error) {
	n, err := nr.readInt()
	if err != nil {
		return "", err
	}
	if n > uint64(maxLength) {
		return "", fmt.Errorf("got string of length %d (max %d in this context)", n, maxLength)
	}
	buf := make([]byte, padStringSize(int(n)))
	if err := nr.read(buf); err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

func (nr *Reader) expect(s string) error {
	n, err := nr.readSmallString()
	if err != nil {
		return err
	}
	// Under gc compiler, string conversion will not allocate.
	// https://github.com/golang/go/wiki/CompilerOptimizations#conversion-for-string-comparison
	if string(nr.buf[:n]) != s {
		return fmt.Errorf("got %q token (expected %q token)", string(nr.buf[:n]), s)
	}
	return nil
}
