package main

import (
	"flag"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
)

func TestGitHubRepoFlag(t *testing.T) {
	parse := func(args ...string) ([]app.GitHubRepo, error) {
		var repos []app.GitHubRepo
		flags := flag.NewFlagSet("inftask", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		flags.Func("github-repo", "", gitHubRepoFlag(&repos))
		return repos, flags.Parse(args)
	}
	repos, err := parse("-github-repo", "default=DengNaichen/inftask", "-github-repo", "acme-2=owner/name")
	if want := []app.GitHubRepo{{Workspace: "default", Repo: "DengNaichen/inftask"}, {Workspace: "acme-2", Repo: "owner/name"}}; err != nil || !slices.Equal(repos, want) {
		t.Fatalf("repos = %#v, err = %v; want %#v", repos, err, want)
	}
	for _, value := range []string{"DengNaichen/inftask", "=DengNaichen/inftask"} {
		if _, err := parse("-github-repo", value); err == nil || !strings.Contains(err.Error(), "<slug>=<owner>/<name>") {
			t.Fatalf("-github-repo %s: err = %v, want the expected format", value, err)
		}
	}
}
