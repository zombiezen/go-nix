package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"zombiezen.com/go/nix"
	"zombiezen.com/go/nix/nar"
)

func newHashCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "hash",
		Short: "Compute and convert cryptographic hashes",
	}
	c.AddCommand(
		newHashFileCommand(),
		newHashPathCommand(),
		newHashToBaseCommand("to-base16", "base-16", nix.Hash.RawBase16),
		newHashToBaseCommand("to-base32", "base-32", nix.Hash.RawBase32),
		newHashToBaseCommand("to-base64", "base-64", nix.Hash.RawBase64),
		newHashToBaseCommand("to-sri", "SRI", nix.Hash.SRI),
	)
	return c
}

func newHashFileCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "file [flags] PATH [...]",
		DisableFlagsInUseLine: true,
		Short:                 "Print cryptographic hash of a regular file",
		Args:                  cobra.MinimumNArgs(1),
		SilenceErrors:         true,
		SilenceUsage:          true,
	}
	hashType := nix.SHA256
	c.Flags().Var((*hashTypeFlag)(&hashType), "type", "hash `algorithm`")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		return runHashFile(cmd.Context(), hashType, args)
	}
	return c
}

func runHashFile(ctx context.Context, typ nix.HashType, files []string) error {
	for _, fname := range files {
		f, err := os.Open(fname)
		if err != nil {
			return err
		}
		h := nix.NewHasher(typ)
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
		digest := h.SumHash()
		fmt.Println(digest)
	}
	return nil
}

func newHashPathCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "path [flags] PATH [...]",
		DisableFlagsInUseLine: true,
		Short:                 "Print cryptographic hash of the NAR serialization of a path",
		Args:                  cobra.MinimumNArgs(1),
		SilenceErrors:         true,
		SilenceUsage:          true,
	}
	hashType := nix.SHA256
	c.Flags().Var((*hashTypeFlag)(&hashType), "type", "hash `algorithm`")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		return runHashFile(cmd.Context(), hashType, args)
	}
	return c
}

func runHashPath(ctx context.Context, typ nix.HashType, files []string) error {
	for _, fname := range files {
		h := nix.NewHasher(typ)
		if err := nar.DumpPath(h, fname); err != nil {
			return err
		}
		digest := h.SumHash()
		fmt.Println(digest)
	}
	return nil
}

func newHashToBaseCommand(use string, repr string, format func(nix.Hash) string) *cobra.Command {
	c := &cobra.Command{
		Use:                   use + " [flags] STRING [...]",
		DisableFlagsInUseLine: true,
		Short:                 "Convert hash(es) to a " + repr + " representation",
		Args:                  cobra.MinimumNArgs(1),
		SilenceErrors:         true,
		SilenceUsage:          true,
	}
	hashType := nix.SHA256
	c.Flags().Var((*hashTypeFlag)(&hashType), "type", "hash `algorithm`")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		return runHashToBase(cmd.Context(), hashType, args, format)
	}
	return c
}

func runHashToBase(ctx context.Context, typ nix.HashType, hashStrings []string, format func(nix.Hash) string) error {
	for _, s := range hashStrings {
		h, err := nix.ParseHash(s)
		if err != nil {
			return err
		}
		fmt.Println(format(h))
	}
	return nil
}

type hashTypeFlag nix.HashType

func (htf *hashTypeFlag) String() string {
	return nix.HashType(*htf).String()
}

func (htf *hashTypeFlag) Set(s string) error {
	typ, err := nix.ParseHashType(s)
	if err != nil {
		return err
	}
	*htf = hashTypeFlag(typ)
	return nil
}

func (htf *hashTypeFlag) Type() string {
	return "hash type"
}
