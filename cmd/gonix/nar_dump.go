package main

import (
	"bufio"
	"context"
	"os"

	"github.com/spf13/cobra"
	"zombiezen.com/go/nix/nar"
)

func newNARDumpCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "dump PATH",
		DisableFlagsInUseLine: true,
		Short:                 "Serialise a path to stdout in NAR format",
		Args:                  cobra.ExactArgs(1),
		SilenceErrors:         true,
		SilenceUsage:          true,
	}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		return runNARDump(cmd.Context(), args[0])
	}
	return c
}

func runNARDump(ctx context.Context, file string) error {
	// grab stdout
	w := bufio.NewWriter(os.Stdout)

	err := nar.DumpPath(w, file)
	if err != nil {
		return err
	}

	return w.Flush()
}
