package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/w00j00ng/goproj/tdelta/cmd"
)

func main() {
	// Let writes to a closed stdout pipe return an error to the CLI.
	signal.Notify(make(chan os.Signal, 1), syscall.SIGPIPE)
	os.Exit(cmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
