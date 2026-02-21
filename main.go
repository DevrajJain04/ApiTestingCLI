package main

import (
	"os"

	"github.com/DevrajJain04/reqres/internal/cli"
)

func main() {
	exitCode := cli.Run(os.Args[1:])
	os.Exit(exitCode)
}
