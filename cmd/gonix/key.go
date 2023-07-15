package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"zombiezen.com/go/nix"
)

func newKeyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "key",
		Short: "Compute and convert cryptographic hashes",
	}
	c.AddCommand(
		newKeyConvertSecretToPublicCommand(),
		newKeyGenerateSecretCommand(),
	)
	return c
}

func newKeyGenerateSecretCommand() *cobra.Command {
	c := &cobra.Command{
		Use:                   "generate-secret [flags] --key-name=NAME",
		DisableFlagsInUseLine: true,
		Short:                 "Generate a secret key for signing store paths",
		Args:                  cobra.NoArgs,
		SilenceErrors:         true,
		SilenceUsage:          true,
	}
	name := c.Flags().String("key-name", "", "`identifier` of the key (e.g. cache.example.org-1)")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if *name == "" {
			return fmt.Errorf("--key-name missing")
		}
		return runKeyGenerateSecret(cmd.Context(), *name)
	}
	return c
}

func runKeyGenerateSecret(ctx context.Context, name string) error {
	_, key, err := nix.GenerateKey(name, nil)
	if err != nil {
		return err
	}
	fmt.Println(key)
	return nil
}

func newKeyConvertSecretToPublicCommand() *cobra.Command {
	c := &cobra.Command{
		Use:           "convert-secret-to-public",
		Short:         "Generate a public key for verifying store paths from a secret key read from standard input",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		return runKeyConvertSecretToPublic(cmd.Context())
	}
	return c
}

func runKeyConvertSecretToPublic(ctx context.Context) error {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	input = bytes.TrimSpace(input)
	key := new(nix.PrivateKey)
	if err := key.UnmarshalText(input); err != nil {
		return err
	}
	fmt.Println(key.PublicKey())
	return nil
}
