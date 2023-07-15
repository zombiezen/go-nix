package nar

import (
	"fmt"
	"io"
	"os"

	"zombiezen.com/go/nix/nar"
)

type CatCmd struct {
	Nar  string `kong:"arg,type='existingfile',help='Path to the NAR'"`
	Path string `kong:"arg,type='string',help='Path inside the NAR, starting with \"/\".'"`
}

func (cmd *CatCmd) Run() error {
	f, err := os.Open(cmd.Nar)
	if err != nil {
		return err
	}

	nr := nar.NewReader(f)
	for {
		hdr, err := nr.Next()
		if err != nil {
			// io.EOF means we didn't find the requested path
			if err == io.EOF {
				return fmt.Errorf("requested path not found")
			}
			// relay other errors
			return err
		}

		if "/"+hdr.Path == cmd.Path {
			if hdr.Mode.Type() != 0 {
				return fmt.Errorf("unable to cat non-regular file")
			}
			break
		}
	}
	// we can't cat directories and symlinks

	_, err = io.Copy(os.Stdout, nr)
	return err
}
