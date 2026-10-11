package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/DengNaichen/inftask/internal/app"
)

// importLinear runs `inftask import-linear`. It opens and migrates the
// database as the server does, so the server using it must be stopped, then
// imports a Linear export into one team in one transaction. It returns the
// exit code.
func importLinear(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("import-linear", flag.ContinueOnError)
	flags.SetOutput(stderr)
	data := flags.String("data", "", "SQLite database path")
	workspace := flags.String("workspace", "", "slug of the workspace to import into")
	team := flags.String("team", "", "key of the existing team the issues belong to")
	from := flags.String("from", "", "directory written by scripts/linear/export.py")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *data == "" || *workspace == "" || *team == "" || *from == "" || flags.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: inftask import-linear -data <database> -workspace <slug> -team <key> -from <export directory>")
		return 2
	}
	service, err := app.Open(*data)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer service.Close()
	ctx := context.Background()
	scoped, err := service.InWorkspace(ctx, *workspace)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	result, err := scoped.ImportLinear(ctx, *team, *from)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "imported %d issues, %d comments, %d relations, %d links into %s; created %d Projects\n",
		result.Issues, result.Comments, result.Relations, result.Links, *team, result.Projects)
	return 0
}
