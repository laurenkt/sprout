package main

import (
	"os"
	
	"sprout/pkg/presentation/cli"
)

func main() {
	exitCode := cli.Run(os.Args)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
