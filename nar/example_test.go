package nar_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"zombiezen.com/go/nix/nar"
)

func ExampleReader() {
	// Open a NAR file for reading.
	narFile, err := os.Open(filepath.Join("testdata", "mini-drv.nar"))
	if err != nil {
		log.Fatal(err)
	}
	defer narFile.Close()

	// List the NAR file's contents.
	// To reduce I/O overhead, we wrap the OS file with a buffered reader.
	narReader := nar.NewReader(bufio.NewReader(narFile))
	for {
		hdr, err := narReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if hdr.Path == "" {
			// Root directory.
			fmt.Printf("%v %3d\n", hdr.Mode, hdr.Size)
		} else {
			fmt.Printf("%v %3d %s\n", hdr.Mode, hdr.Size, hdr.Path)
		}
	}
	// Output:
	// dr-xr-xr-x   0
	// -r--r--r--   4 a.txt
	// dr-xr-xr-x   0 bin
	// -r-xr-xr-x  45 bin/hello.sh
	// -r--r--r--  14 hello.txt
}

func ExampleFS() {
	// Open a NAR file for reading.
	// Importantly, *os.File implements both io.Reader and io.ReaderAt.
	narFile, err := os.Open(filepath.Join("testdata", "mini-drv.nar"))
	if err != nil {
		log.Fatal(err)
	}
	defer narFile.Close()

	// Index the NAR file for random access.
	listing, err := nar.List(narFile)
	if err != nil {
		log.Fatal(err)
	}

	// FS allows us to operate on the NAR archive
	// using the standard operations in io/fs.
	fsys, err := nar.NewFS(narFile, listing)
	if err != nil {
		log.Fatal(err)
	}
	data, err := fs.ReadFile(fsys, "hello.txt")
	if err != nil {
		log.Fatal(err)
	}
	os.Stdout.Write(data)
	// Output:
	// Hello, World!
}

func ExampleWriter() {
	// Set up a Writer that writes to buf.
	buf := new(bytes.Buffer)
	narWriter := nar.NewWriter(buf)

	// Create a file, hello.txt.
	// The root of the archive will automatically be a directory.
	const fileContent = "Hello, World!\n"
	err := narWriter.WriteHeader(&nar.Header{
		Path: "hello.txt",
		Size: int64(len(fileContent)),
	})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := io.WriteString(narWriter, fileContent); err != nil {
		log.Fatal(err)
	}

	// To complete the archive, you must call Close.
	// Otherwise, you end up with a corrupted file.
	if err := narWriter.Close(); err != nil {
		log.Fatal(err)
	}
}
