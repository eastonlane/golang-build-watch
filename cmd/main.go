package main

import (
	"fmt"
	"os"

	"golang-build-watch/internal/build"
	"golang-build-watch/internal/cli"
	"golang-build-watch/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "__toolexec" {
		if err := build.RunToolExec(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := cli.NewRootCmd(version.String()).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
