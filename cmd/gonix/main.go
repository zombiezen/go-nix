package main

import (
	"os"

	"github.com/alecthomas/kong"
	"zombiezen.com/go/nix/cmd/gonix/nar"
)

//nolint:gochecknoglobals
var cli struct {
	Nar nar.Cmd `kong:"cmd,name='nar',help='Create or inspect NAR files'"`
}

func main() {
	parser, err := kong.New(&cli)
	if err != nil {
		panic(err)
	}

	ctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		panic(err)
	}
	// Call the Run() method of the selected parsed command.
	err = ctx.Run()

	ctx.FatalIfErrorf(err)
}
