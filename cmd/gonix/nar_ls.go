package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"zombiezen.com/go/nix/nar"
)

func newNARListCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "ls [-R] ARCHIVE [PATH]",
		DisableFlagsInUseLine: false,
		Short:                 "Show information about a path inside a NAR file",
		Args:                  cobra.RangeArgs(1, 2),
		SilenceErrors:         true,
		SilenceUsage:          true,
	}
	recursive := c.Flags().BoolP("recursive", "R", false, "Whether to list recursively, or only the current level.")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		fileArg := "/"
		if len(args) > 1 {
			fileArg = args[1]
		}
		return runNARList(cmd.Context(), args[0], fileArg, *recursive)
	}
	return c
}

// headerLineString returns a one-line string describing a header.
// hdr.Validate() is assumed to be true.
func headerLineString(hdr *nar.Header) string {
	var sb strings.Builder

	sb.WriteString(hdr.FileInfo().Mode().String())
	sb.WriteString(" /")
	sb.WriteString(hdr.Path)

	// if regular file, show size in parantheses. We don't bother about aligning it nicely,
	// as that'd require reading in all headers first before printing them out.
	if hdr.Size > 0 {
		sb.WriteString(fmt.Sprintf(" (%v bytes)", hdr.Size))
	}

	// if LinkTarget, show it
	if hdr.LinkTarget != "" {
		sb.WriteString(" -> ")
		sb.WriteString(hdr.LinkTarget)
	}

	sb.WriteString("\n")

	return sb.String()
}

func runNARList(ctx context.Context, archivePath string, file string, recursive bool) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}

	nr := nar.NewReader(f)
	for {
		hdr, err := nr.Next()
		if err != nil {
			// io.EOF means we're done
			if err == io.EOF {
				return nil
			}
			// relay other errors
			return err
		}

		// if the yielded path starts with the path specified
		if strings.HasPrefix("/"+hdr.Path, file) {
			remainder := hdr.Path[len(file)-1:]
			// If recursive was requested, return all these elements.
			// Else, look at the remainder - There may be no other slashes.
			if recursive || !strings.Contains(remainder, "/") {
				// fmt.Printf("%v type %v\n", hdr.Type, hdr.Path)
				print(headerLineString(hdr))
			}
		} else {
			// We can exit early as soon as we receive a header whose path doesn't have the prefix we're searching for,
			// and the path is lexicographically bigger than our search prefix
			if "/"+hdr.Path > file {
				return nil
			}
		}
	}
}
