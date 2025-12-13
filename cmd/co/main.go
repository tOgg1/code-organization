package main

import (
	"os"

	"github.com/tormodhaugland/co/cmd/co/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
