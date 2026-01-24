package main

import (
	"fmt"
	"io"
	"os"

	"github.com/unraid/apprise-go/internal/version"
)

const usageText = "" +
	"Usage:\n" +
	"   apprise [OPTIONS] [APPRISE_URL [APPRISE_URL2 [APPRISE_URL3]]]\n" +
	"   apprise storage [OPTIONS] [ACTION] [UID1 [UID2 [UID3]]]\n"

func main() {
	args := os.Args[1:]

	if hasArg(args, "-V", "--version") {
		fmt.Fprintln(os.Stdout, version.Message())
		return
	}

	if len(args) == 0 || hasArg(args, "-h", "--help") {
		printUsage(os.Stdout)
		if len(args) == 0 {
			os.Exit(1)
		}
		return
	}

	fmt.Fprintln(os.Stderr, "apprise-go: CLI not implemented yet")
	os.Exit(2)
}

func hasArg(args []string, flags ...string) bool {
	if len(args) == 0 || len(flags) == 0 {
		return false
	}

	for _, arg := range args {
		for _, flag := range flags {
			if arg == flag {
				return true
			}
		}
	}

	return false
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, usageText)
}
