package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/DengNaichen/inftask/internal/app"
	"io"
	"os"
)

func bootstrapAccess(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("bootstrap-access", flag.ContinueOnError)
	flags.SetOutput(stderr)
	data := flags.String("data", "", "Existing SQLite database path")
	workspace := flags.String("workspace", "", "Existing workspace slug")
	subject := flags.String("subject", "", "Verified UC subject of the initial administrator")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *data == "" || *workspace == "" || *subject == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "-data, -workspace and -subject are required")
		return 2
	}
	if _, err := os.Stat(*data); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	service, err := app.Open(*data)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer service.Close()
	if err := service.BootstrapAccess(context.Background(), *workspace, *subject); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "Initialized administrator for workspace %s\n", *workspace)
	return 0
}
