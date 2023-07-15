package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"zombiezen.com/go/bass/sigterm"
)

func main() {
	rootCommand := &cobra.Command{
		Use:           "gonix",
		Short:         "Go Nix test CLI",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	narGroup := &cobra.Command{
		Use:   "nar",
		Short: "Create or inspect NAR files",
	}
	narGroup.AddCommand(
		newNARCatCommand(),
		newNARDumpCommand(),
		newNARListCommand(),
	)

	rootCommand.AddCommand(
		narGroup,
		newHashCommand(),
		newKeyCommand(),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), sigterm.Signals()...)
	err := rootCommand.ExecuteContext(ctx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gonix:", err)
		os.Exit(1)
	}
}
