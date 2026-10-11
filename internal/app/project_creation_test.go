package app

import (
	"context"
	"errors"
	"testing"
)

func TestProjectCreationValidation(t *testing.T) {
	s := labelService(t)
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, " \t ", "", "AL"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("blank name: %v", err)
	}
	p, err := s.CreateProject(ctx, " Release ", "Outcome", "AL")
	if err != nil || p.Name != "Release" {
		t.Fatalf("trim: %#v %v", p, err)
	}
	if _, err = s.CreateProject(ctx, "Release", "other", "AL"); !errors.Is(err, ErrProjectExists) {
		t.Fatalf("duplicate: %v", err)
	}
	ps, err := s.ListProjects(ctx, true)
	if err != nil || len(ps) != 1 || ps[0].Description != "Outcome" {
		t.Fatalf("partial create: %#v %v", ps, err)
	}
}
