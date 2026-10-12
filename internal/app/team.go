package app

import (
	"context"
	"regexp"
	"strings"
)

var teamKey = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,5}$`)

type Team struct {
	Private           bool   `json:"private"`
	Joined            bool   `json:"joined"`
	Accessible        bool   `json:"accessible"`
	CanManage         bool   `json:"can_manage"`
	MembershipEnabled bool   `json:"membership_enabled"`
	Emoji             string `json:"emoji" jsonschema:"emoji icon; empty uses the default icon"`
	Key               string `json:"key" jsonschema:"identifier prefix; fixed once created"`
	Name              string `json:"name"`
	NextNumber        int    `json:"next_number" jsonschema:"number the team's next issue gets"`
}

type CreateTeam struct {
	Private     bool   `json:"private,omitempty" jsonschema:"make this team private; creator joins as owner"`
	Emoji       string `json:"emoji,omitempty" jsonschema:"optional Unicode emoji icon; empty uses the default icon"`
	Key         string `json:"key" jsonschema:"2 to 6 uppercase letters or digits starting with a letter, such as AL; cannot be changed later"`
	Name        string `json:"name" jsonschema:"required nonempty team name"`
	StartNumber *int   `json:"start_number,omitempty" jsonschema:"number the team's first issue gets, at least 1; omitted starts at 1"`
}

type UpdateTeam struct {
	Private *bool   `json:"private,omitempty" jsonschema:"change visibility; owner or administrator only"`
	Name    *string `json:"name,omitempty" jsonschema:"new display name; trimmed and nonempty; omitted leaves unchanged"`
	Key     string  `json:"key" jsonschema:"existing team key; key and numbering cannot change"`
	Emoji   *string `json:"emoji,omitempty" jsonschema:"Unicode emoji icon; omitted leaves unchanged, empty clears"`
}

func (s *Service) ListTeams(ctx context.Context) ([]Team, error) {
	if err := s.checkAccess(ctx); err != nil {
		return nil, err
	}
	return s.teams(ctx, false)
}

// CreateTeam adds a team whose numbering starts at StartNumber, or 1 when it
// is omitted. A team's key and numbering cannot be changed.
func (s *Service) CreateTeam(ctx context.Context, input CreateTeam) (Team, error) {
	if err := s.checkAccess(ctx); err != nil {
		return Team{}, err
	}
	team := Team{Key: input.Key, Name: strings.TrimSpace(input.Name), NextNumber: 1, Emoji: input.Emoji, Private: input.Private}
	if !validEmoji(input.Emoji) {
		return Team{}, ErrInvalidEmoji
	}
	if input.StartNumber != nil {
		team.NextNumber = *input.StartNumber
	}
	if !teamKey.MatchString(team.Key) || team.Name == "" || team.NextNumber < 1 {
		return Team{}, ErrInvalidTeam
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Team{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO teams(workspace_id, key, name, next_number, emoji, private) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(workspace_id, key) DO NOTHING`,
		s.workspace, team.Key, team.Name, team.NextNumber, team.Emoji, team.Private)
	if err != nil {
		return Team{}, err
	}
	if inserted, _ := result.RowsAffected(); inserted == 0 {
		return Team{}, ErrTeamExists
	}
	if identity := requestIdentity(ctx); identity.Subject != "" {
		if err := saveIdentity(ctx, tx, identity); err != nil {
			return Team{}, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return Team{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO team_members(team_id,subject,role) VALUES(?,?,'owner')`, id, identity.Subject); err != nil {
			return Team{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Team{}, err
	}
	return s.teamSettings(ctx, team.Key)
}

func (s *Service) UpdateTeam(ctx context.Context, input UpdateTeam) (Team, error) {
	if err := s.checkAccess(ctx); err != nil {
		return Team{}, err
	}
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
	team, err := s.teamSettings(ctx, input.Key)
	if err != nil {
		return Team{}, err
	}
	if input.Private != nil && team.MembershipEnabled && !team.CanManage {
		return Team{}, ErrTeamOwnerRequired
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Team{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `UPDATE teams SET name=COALESCE(?,name),emoji=COALESCE(?,emoji),private=COALESCE(?,private) WHERE workspace_id=? AND key=?`, input.Name, input.Emoji, input.Private, s.workspace, input.Key)
	if err != nil {
		return Team{}, err
	}
	if input.Private != nil && *input.Private && !team.Private && team.MembershipEnabled {
		_, err = tx.ExecContext(ctx, `INSERT INTO team_members(team_id,subject,role) SELECT id,?,'owner' FROM teams WHERE workspace_id=? AND key=? ON CONFLICT(team_id,subject) DO UPDATE SET role='owner'`, requestIdentity(ctx).Subject, s.workspace, input.Key)
		if err != nil {
			return Team{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Team{}, err
	}
	return s.teamSettings(ctx, input.Key)
}
