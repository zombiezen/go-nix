package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"zombiezen.com/go/nix/nar"
)

func newNARCatCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "cat ARCHIVE FILE",
		DisableFlagsInUseLine: true,
		Short:                 "Print the contents of a file inside a NAR file",
		Args:                  cobra.ExactArgs(2),
		SilenceErrors:         true,
		SilenceUsage:          true,
	}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		fileArg := "/"
		if len(args) > 1 {
			fileArg = args[1]
		}
		return runNARCat(cmd.Context(), args[0], fileArg)
	}
	return c
}

func runNARCat(ctx context.Context, archivePath string, file string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

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

		if "/"+hdr.Path == file {
			if !hdr.Mode.IsRegular() {
				return fmt.Errorf("unable to cat non-regular file")
			}
			break
		}
	}

	_, err = io.Copy(os.Stdout, nr)
	return err
}
