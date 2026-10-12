package main

import (
	"bytes"
	"context"
	"github.com/DengNaichen/inftask/internal/app"
	"path/filepath"
	"testing"
)

func TestBootstrapAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	service, err := app.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateWorkspace(context.Background(), app.Workspace{Slug: "existing", Name: "Existing"}); err != nil {
		t.Fatal(err)
	}
	service.Close()
	var out, stderr bytes.Buffer
	if code := bootstrapAccess([]string{"-data", path, "-workspace", "existing"}, &out, &stderr); code != 2 {
		t.Fatal(code)
	}
	args := []string{"-data", path, "-workspace", "existing", "-subject", "chosen-user"}
	if code := bootstrapAccess(args, &out, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	if code := bootstrapAccess(args, &out, &stderr); code != 1 {
		t.Fatal("rebootstrap accepted", code)
	}
	service, err = app.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := app.WithIdentity(context.Background(), app.Identity{Subject: "chosen-user"})
	scoped, err := service.ForRequests(true).InWorkspace(ctx, "existing")
	if err != nil {
		t.Fatal(err)
	}
	access, err := scoped.Access(ctx)
	if err != nil || access.Role != "admin" {
		t.Fatal(access, err)
	}
}
