package nix_test

import (
	"fmt"

	"zombiezen.com/go/nix"
)

func ExampleStorePath_Digest() {
	path, err := nix.ParseStorePath("/nix/store/s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1")
	if err != nil {
		panic(err)
	}
	fmt.Println(path.Digest())
	// Output:
	// s66mzxpvicwk07gjbjfw9izjfa797vsw
}

func ExampleStorePath_Name() {
	path, err := nix.ParseStorePath("/nix/store/s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1")
	if err != nil {
		panic(err)
	}
	fmt.Println(path.Name())
	// Output:
	// hello-2.12.1
}

func ExampleStoreDirectory_ParsePath() {
	const path = "/nix/store/s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1/bin/hello"
	storePath, subPath, err := nix.DefaultStoreDirectory.ParsePath(path)
	if err != nil {
		panic(err)
	}
	fmt.Println(storePath)
	fmt.Println(subPath)
	// Output:
	// /nix/store/s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1
	// bin/hello
}
