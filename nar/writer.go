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

// ErrNotSparseAllocatable is returned with the provider io.Writer to Writer
// can't be turned into a sparsely-allocatable type. Currently this means it *must*
// be backed by an *os.File
var ErrNotSparseAllocatable = errors.New("nar: underlying writer does not support sparse allocation")

const (
	writerStateInit int8 = iota
	writerStateRoot
	writerStateFile
	writerStateSpecial
	writerStateEnd
)

type WriterOptions func(options *Writer)

type writerOptions struct {
	// sparseAllocate when true causes WriteHeader to also trigger immediate allocation
	// of space for the file content without actually writing anything. Allocation happens
	// after callback is run. It is an error to call Write when sparseAllocate is enabled.
	sparseAllocate bool
	// allocateCallback provides a callback which is run _after_ the writer has allocated space
	// for content.
	allocateCallback func(header Header) error
}

func SparseAllocate(allocateCallback func(header Header) error) WriterOptions {
	return func(w *Writer) {
		w.sparseAllocate = true
		w.allocateCallback = allocateCallback
	}
}

// Writer provides sequential writing of a NAR archive.
// [Writer.WriteHeader] begins a new file with the provided [Header],
// and then Writer can be treated as an [io.Writer] to supply that file's data.
type Writer struct {
	bw BufWriter

	state int8
	// lastPathDir is true if the path named by lastPath is a directory.
	lastPathDir bool
	// remaining is the number of bytes left in a regular file.
	remaining int64
	// lastPath is the path of the last file system object written to the archive.
	lastPath string
	// lastHeader is the last file header which was written to the archive by WriteHeader
	lastHeader Header
	// writerOptions are user modifiable configurables to the behavior of the writer
	writerOptions
}

// NewWriter returns a new [Writer] writing to w.
func NewWriter(w io.Writer, writerOptions ...WriterOptions) *Writer {
	narWriter := &Writer{bw: BufWriter{w: w}}
	for _, opt := range writerOptions {
		opt(narWriter)
	}

	return narWriter
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
	if err := ValidatePath(hdr.Path); err != nil {
		return fmt.Errorf("nar: %w", err)
	}

	switch nw.state {
	case writerStateInit:
		nw.bw.String(Magic)
		nw.state = writerStateRoot
		fallthrough
	case writerStateRoot:
		if hdr.Path != "" {
			nw.bw.String("(")
			nw.bw.String(TypeToken)
			nw.bw.String(TypeDirectory)
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

		// In sparse allocation mode, calling Write() is not allowed - so we need
		// to allocate the file here.
		if nw.remaining > 0 && nw.sparseAllocate {
			if err := nw.allocateFile(); err != nil {
				return err
			}
		}

		if nw.remaining > 0 {
			return fmt.Errorf("nar: %d bytes remaining on %s", nw.remaining, FormatLastPath(nw.lastPath))
		}
		if err := nw.finishFile(); err != nil {
			return err
		}
		nw.bw.String(")") // finish directory entry

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

	pop, newDirs, err := TreeDelta(nw.lastPath, nw.lastPathDir, hdr.Path)
	if err != nil {
		return err
	}
	for i := 0; i < pop; i++ {
		nw.bw.String(")") // directory
		nw.bw.String(")") // parent's entry
	}
	for newDirs != "" {
		name := FirstPathComponent(newDirs)
		nw.bw.String(EntryToken)
		nw.bw.String("(")
		nw.bw.String(NameToken)
		nw.bw.String(name)
		nw.bw.String(NodeToken)
		nw.bw.String("(")
		nw.bw.String(TypeToken)
		nw.bw.String(TypeDirectory)

		newDirs = newDirs[len(name):]
		if len(newDirs) >= len("/") {
			newDirs = newDirs[len("/"):]
		}
	}
	if hdr.Path != "" {
		name := slashpath.Base(hdr.Path)
		nw.bw.String(EntryToken)
		nw.bw.String("(")
		nw.bw.String(NameToken)
		nw.bw.String(name)
		nw.bw.String(NodeToken)
	}

	switch hdr.Mode.Type() {
	case 0: // regular
		nw.bw.String("(")
		nw.bw.String(TypeToken)
		nw.bw.String(TypeRegular)
		if hdr.Mode&0o111 != 0 {
			nw.bw.String(ExecutableToken)
			nw.bw.String("")
		}
		nw.bw.String(ContentsToken)
		nw.bw.Uint64(uint64(hdr.Size))
		nw.state = writerStateFile
		nw.remaining = hdr.Size
	case fs.ModeDir:
		nw.bw.String("(")
		nw.bw.String(TypeToken)
		nw.bw.String(TypeDirectory)
		nw.state = writerStateSpecial
	case fs.ModeSymlink:
		nw.bw.String("(")
		nw.bw.String(TypeToken)
		nw.bw.String(TypeSymlink)
		nw.bw.String(TargetToken)
		nw.bw.String(hdr.LinkTarget)
		nw.bw.String(")")
		if hdr.Path != "" {
			nw.bw.String(")")
		}
		nw.state = writerStateSpecial
	default:
		return fmt.Errorf("nar: %s: cannot support mode %v", hdr.Path, hdr.Mode)
	}
	nw.bw.Flush()
	nw.lastPath = hdr.Path
	nw.lastPathDir = hdr.Mode.IsDir()
	// Record the last header
	nw.lastHeader = *hdr
	nw.lastHeader.ContentOffset = nw.Offset()

	return nw.bw.err
}

// allocatable is the interface to check for the operations we need to support
// allocation-type dumping
type allocatable interface {
	Truncate(size int64) error
	Seek(offset int64, whence int) (ret int64, err error)
}

// allocateFile handles allocating space in the NAR file to hold future content when sparseAllocation mode
// is used.
func (nw *Writer) allocateFile() error {
	// Get the position of the start of the file (most likely the user is about to use
	// allocateCallback to start backfilling the data)
	if osFile, ok := nw.bw.w.(allocatable); ok {
		// Expand the file to the new size
		if err := osFile.Truncate(nw.bw.off + nw.remaining); err != nil {
			return err
		}
		// Seek to the new logically empty location
		if _, err := osFile.Seek(nw.remaining, io.SeekCurrent); err != nil {
			return err
		}
		// We have now finished "writing"
		nw.bw.off += nw.remaining
		nw.remaining = 0
		if nw.allocateCallback != nil {
			// Call the allocateCallback so the user can either populate the file or
			// record it for later or whatever they're doing.
			if err := nw.allocateCallback(nw.lastHeader); err != nil {
				return err
			}
		}
	} else {
		// Error: can't sparse allocate with something which doesn't support giving us a file path.
		return ErrNotSparseAllocatable
	}
	return nil
}

// Write writes to the current file in the NAR archive.
// Write returns the error [ErrWriteTooLong]
// if more than Header.Size bytes are written after WriteHeader.
//
// Calling Write on special types like [fs.ModeDir] and [fs.ModeSymlink]
// returns (0, [ErrWriteTooLong]) regardless of what the Header.Size claims.
// Calling Write on a writer doing sparse allocation _always_ returns
// (0, [ErrWriteTooLong]) since the write is always completed in the header.
func (nw *Writer) Write(p []byte) (n int, err error) {
	if nw.sparseAllocate {
		return 0, ErrWriteTooLong
	}
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
		// In sparse allocation mode, calling Write() is not allowed - so we need
		// to allocate the file here.
		if nw.remaining > 0 && nw.sparseAllocate {
			if err := nw.allocateFile(); err != nil {
				return err
			}
		}

		if nw.remaining > 0 {
			return fmt.Errorf("nar: close: %d bytes remaining on %s", nw.remaining, FormatLastPath(nw.lastPath))
		}
		nw.finishFile()
		if nw.lastPath != "" {
			nw.bw.String(")") // finish directory entry
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
		nw.bw.String(")") // directory
		nw.bw.String(")") // parent's entry
	}
	if nw.lastPath != "" || nw.lastPathDir {
		nw.bw.String(")") // root directory
	}

	nw.bw.Flush()
	prevErr := nw.bw.err
	if nw.bw.err == nil {
		nw.bw.err = errors.New("nar: writer closed")
	}
	return prevErr
}

func (nw *Writer) finishFile() error {
	nw.bw.Pad()
	nw.bw.String(")")
	return nw.bw.err
}

// BufWriter implements buffered NAR string writing.
type BufWriter struct {
	w io.Writer
	// off is the number of bytes written to w.
	// It does not include bytes written to buf.
	off int64
	// err is the first error returned by w.
	err error
	// buf is a temporary buffer used for writing.
	// Its length is a multiple of StringAlign
	// that is sufficient to hold any of the known tokens in the NAR format.
	buf    [256]byte
	bufLen int16
}

// Write passes through a write to the underlying writer.
func (bw *BufWriter) Write(p []byte) (n int, err error) {
	bw.Flush()
	if bw.err != nil {
		return 0, bw.err
	}
	n, err = bw.w.Write(p)
	bw.off += int64(n)
	return n, err
}

// Flush writes any buffered data to the underlying writer.
func (bw *BufWriter) Flush() {
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

// Uint64 writes a little-endian 64-bit integer.
func (bw *BufWriter) Uint64(x uint64) {
	if bw.err != nil {
		return
	}
	bw.Pad()
	if int(bw.bufLen)+8 > len(bw.buf) {
		bw.Flush()
		if bw.err != nil {
			return
		}
	}
	binary.LittleEndian.PutUint64(bw.buf[bw.bufLen:], x)
	bw.bufLen += 8
}

// String writes a String prefixed by its length.
func (bw *BufWriter) String(s string) {
	bw.Uint64(uint64(len(s)))
	if len(s) == 0 || bw.err != nil {
		return
	}
	n := PadStringSize(len(s))
	if n > len(bw.buf) {
		bw.LongString(s)
		return
	}

	if int(bw.bufLen)+n > len(bw.buf) {
		// String *will* fit in buffer once flushed.
		nn := copy(bw.buf[bw.bufLen:], s)
		s = s[nn:]
		bw.bufLen += int16(nn)
		for ; bw.bufLen < int16(len(bw.buf)); bw.bufLen++ {
			bw.buf[bw.bufLen] = 0
		}
		bw.Flush()
		if bw.err != nil {
			return
		}
	}

	// String fits in buffer.
	nn := copy(bw.buf[bw.bufLen:], s)
	bw.bufLen += int16(nn)
	bw.Pad()
}

func (bw *BufWriter) LongString(s string) {
	// Less common case: string/token does not fit in buffer.
	// Try to use WriteString if possible, otherwise multiple Writes.

	if sw, ok := bw.w.(io.StringWriter); ok {
		bw.Flush()
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
				bw.Flush()
				if bw.err != nil {
					return
				}
			}
			n := copy(bw.buf[bw.bufLen:], s[i:])
			bw.bufLen += int16(n)
			i += n
		}
	}

	bw.Pad()
}

// Pad writes zero bytes until bw.off+bw.bufLen is evenly divisible by StringAlign.
func (bw *BufWriter) Pad() {
	if bw.err != nil {
		return
	}
	n := StringPaddingLength(int(bw.off%StringAlign) + int(bw.bufLen))
	if int(bw.bufLen)+n > len(bw.buf) {
		n -= len(bw.buf) - int(bw.bufLen)
		for ; int(bw.bufLen) < len(bw.buf); bw.bufLen++ {
			bw.buf[bw.bufLen] = 0
		}
		bw.Flush()
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

// TreeDelta computes the directory ends (pops) and/or new directories to be created
// in order to advance from one path to another.
func TreeDelta(oldPath string, oldIsDir bool, newPath string) (pop int, newDirs string, err error) {
	newParent, _ := slashpath.Split(newPath)
	if shared := oldPath + "/"; strings.HasPrefix(newPath, shared) {
		if !oldIsDir {
			return 0, "", fmt.Errorf("%s is not a directory", FormatLastPath(oldPath))
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
		newName := FirstPathComponent(newPath[len(shared):])
		oldName := FirstPathComponent(oldPath[len(shared):])
		if newName <= oldName {
			return 0, "", fmt.Errorf("%s is not ordered after %s",
				FormatLastPath(newPath[:len(shared)+len(newName)]),
				FormatLastPath(oldPath[:len(shared)+len(oldName)]))
		}
	}

	newDirs = strings.TrimSuffix(newParent[len(shared):], "/")
	return pop, newDirs, nil
}

func FirstPathComponent(path string) string {
	i := strings.IndexByte(path, '/')
	if i == -1 {
		return path
	}
	return path[:i]
}

func FormatLastPath(s string) string {
	if s == "" {
		return "<root filesystem object>"
	}
	return s
}

func ValidatePath(origPath string) error {
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
		if err := ValidateFilename(path[:elemEnd]); err != nil {
			return err
		}
		if elemEnd == len(path) {
			return nil
		}
		path = path[elemEnd+1:]
	}
}
