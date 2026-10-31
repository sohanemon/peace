package main

import (
	"fmt"
	"os"

	"io.github.sohanemon/peace/internal/peace"
)

func main() {
	if err := peace.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
