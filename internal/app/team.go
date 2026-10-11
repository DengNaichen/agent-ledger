package app

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
)

var teamKey = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,5}$`)

type Team struct {
	Emoji      string `json:"emoji" jsonschema:"emoji icon; empty uses the default icon"`
	Key        string `json:"key" jsonschema:"identifier prefix; fixed once created"`
	Name       string `json:"name"`
	NextNumber int    `json:"next_number" jsonschema:"number the team's next issue gets"`
}

type CreateTeam struct {
	Emoji       string `json:"emoji,omitempty" jsonschema:"optional Unicode emoji icon; empty uses the default icon"`
	Key         string `json:"key" jsonschema:"2 to 6 uppercase letters or digits starting with a letter, such as AL; cannot be changed later"`
	Name        string `json:"name" jsonschema:"required nonempty team name"`
	StartNumber *int   `json:"start_number,omitempty" jsonschema:"number the team's first issue gets, at least 1; omitted starts at 1"`
}

type UpdateTeam struct {
	Name  *string `json:"name,omitempty" jsonschema:"new display name; trimmed and nonempty; omitted leaves unchanged"`
	Key   string  `json:"key" jsonschema:"existing team key; key and numbering cannot change"`
	Emoji *string `json:"emoji,omitempty" jsonschema:"Unicode emoji icon; omitted leaves unchanged, empty clears"`
}

func (s *Service) ListTeams(ctx context.Context) ([]Team, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, name, next_number, emoji FROM teams WHERE workspace_id = ? ORDER BY key`, s.workspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	teams := []Team{}
	for rows.Next() {
		var team Team
		if err := rows.Scan(&team.Key, &team.Name, &team.NextNumber, &team.Emoji); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

// CreateTeam adds a team whose numbering starts at StartNumber, or 1 when it
// is omitted. A team's key and numbering cannot be changed.
func (s *Service) CreateTeam(ctx context.Context, input CreateTeam) (Team, error) {
	team := Team{Key: input.Key, Name: strings.TrimSpace(input.Name), NextNumber: 1, Emoji: input.Emoji}
	if !validEmoji(input.Emoji) {
		return Team{}, ErrInvalidEmoji
	}
	if input.StartNumber != nil {
		team.NextNumber = *input.StartNumber
	}
	if !teamKey.MatchString(team.Key) || team.Name == "" || team.NextNumber < 1 {
		return Team{}, ErrInvalidTeam
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO teams(workspace_id, key, name, next_number, emoji) VALUES (?, ?, ?, ?, ?) ON CONFLICT(workspace_id, key) DO NOTHING`,
		s.workspace, team.Key, team.Name, team.NextNumber, team.Emoji)
	if err != nil {
		return Team{}, err
	}
	if inserted, _ := result.RowsAffected(); inserted == 0 {
		return Team{}, ErrTeamExists
	}
	return team, nil
}

func (s *Service) UpdateTeam(ctx context.Context, input UpdateTeam) (Team, error) {
	if input.Emoji != nil && !validEmoji(*input.Emoji) {
		return Team{}, ErrInvalidEmoji
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return Team{}, ErrInvalidName
		}
		input.Name = &name
	}
	var team Team
	err := s.db.QueryRowContext(ctx, `UPDATE teams SET name = COALESCE(?, name), emoji = COALESCE(?, emoji) WHERE workspace_id = ? AND key = ? RETURNING key, name, next_number, emoji`, input.Name, input.Emoji, s.workspace, input.Key).Scan(&team.Key, &team.Name, &team.NextNumber, &team.Emoji)
	if errors.Is(err, sql.ErrNoRows) {
		return Team{}, ErrTeamNotFound
	}
	return team, err
}
