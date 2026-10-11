package app

import (
	"errors"
	"slices"
)

var (
	ErrNotFound        = errors.New("issue not found")
	ErrProjectNotFound = errors.New("project not found")
	ErrInvalidTitle    = errors.New("title is required")
	ErrInvalidStatus   = errors.New("invalid status: use " + joinOr(statusNames()))
	ErrInvalidPriority = errors.New("invalid priority: use 0 (no priority), 1 (urgent), 2 (high), 3 (medium), or 4 (low)")
	ErrInvalidSort     = errors.New("invalid sort: use priority or updated")
	ErrInvalidType     = errors.New("invalid type: use " + joinOr(append(slices.Clone(Types), "an empty string")))
	ErrInvalidKind     = errors.New("invalid relation kind: use blocks, blocked_by, or related")
	ErrTeamRequired    = errors.New("team is required")
	ErrTeamNotFound    = errors.New("team not found")
	ErrInvalidTeam     = errors.New("invalid team: key must be 2 to 6 uppercase letters or digits starting with a letter, name is required, and start_number must be at least 1")
	ErrTeamExists      = errors.New("a team with this key already exists")

	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrInvalidWorkspace  = errors.New("invalid workspace: slug must be 2 to 32 lowercase letters, digits, or hyphens starting with a letter and cannot be api, assets, auth, mcp, w, or webhooks; name is required")
	ErrWorkspaceExists   = errors.New("a workspace with this slug already exists")
)
