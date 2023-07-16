package nix_test

import (
	"fmt"
	"io"

	"zombiezen.com/go/nix"
)

func ExampleHasher() {
	h := nix.NewHasher(nix.SHA256)
	io.WriteString(h, "Hello, World!\n")
	fmt.Println(h.SumHash())
	// Output:
	// sha256-yYwktnfv9Ehgr+pvSTu67FuxxMuyCcb8K7tH9m/yrTE=
}

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

func ExampleStoreDirectory_Object() {
	const objectName = "s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1"
	storePath, err := nix.DefaultStoreDirectory.Object(objectName)
	if err != nil {
		panic(err)
	}
	fmt.Println(storePath)
	// Output:
	// /nix/store/s66mzxpvicwk07gjbjfw9izjfa797vsw-hello-2.12.1
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
