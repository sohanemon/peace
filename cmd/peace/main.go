package main

import (
	"errors"
	"fmt"
	"os"

	"io.github.sohanemon/peace/internal/peace"
)

func main() {
	if err := peace.Run(os.Args[0], os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
