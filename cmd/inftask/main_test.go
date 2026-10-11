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

func TestGitHubRepoFlagCommaSeparated(t *testing.T) {
	var repos []app.GitHubRepo
	flags := flag.NewFlagSet("inftask", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Func("github-repo", "", gitHubRepoFlag(&repos))
	err := flags.Parse([]string{"-github-repo", "default=owner/one,default=owner/two", "-github-repo", "other=owner/three"})
	want := []app.GitHubRepo{{Workspace: "default", Repo: "owner/one"}, {Workspace: "default", Repo: "owner/two"}, {Workspace: "other", Repo: "owner/three"}}
	if err != nil || !slices.Equal(repos, want) {
		t.Fatalf("repos = %#v, err = %v; want %#v", repos, err, want)
	}
}

func TestGitHubRepoFlagInvalidList(t *testing.T) {
	for _, value := range []string{"", ",default=owner/one", "default=owner/one,", "default=owner/one,,other=owner/two", "default=owner/one,owner/two", "default=owner/one,=owner/two"} {
		t.Run(value, func(t *testing.T) {
			repos := []app.GitHubRepo{{Workspace: "existing", Repo: "owner/existing"}}
			before := slices.Clone(repos)
			err := gitHubRepoFlag(&repos)(value)
			if err == nil || !strings.Contains(err.Error(), "<slug>=<owner>/<name>") {
				t.Fatalf("err = %v, want format error", err)
			}
			if !slices.Equal(repos, before) {
				t.Fatalf("invalid list changed repos: %#v", repos)
			}
		})
	}
}
