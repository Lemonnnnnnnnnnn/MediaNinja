package main

import (
	"MediaNinja/cmd"
	"os"
)

func main() {
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "serve", "--open")
	}
	cmd.Execute()
}
