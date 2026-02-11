package main

import (
	"fmt"
	"os"

	"github.com/frankenstein-ai/frank-blog-content-generator/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
