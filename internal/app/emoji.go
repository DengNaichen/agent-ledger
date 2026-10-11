package app

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
)

// Unicode 16.0 fully-qualified sequences, shared with the web picker.
// Source: https://unicode.org/Public/emoji/16.0/emoji-test.txt
// The Unicode data license is in emojidata/LICENSE.
//
//go:embed emojidata/emoji.json
var emojiData []byte

var ErrInvalidEmoji = errors.New("invalid emoji: choose a Unicode emoji or use an empty string to clear")

var emojiValues = func() map[string]bool {
	var entries []struct {
		Emoji string `json:"emoji"`
	}
	if err := json.Unmarshal(emojiData, &entries); err != nil {
		panic(err)
	}
	values := map[string]bool{"": true}
	for _, entry := range entries {
		values[entry.Emoji] = true
	}
	return values
}()

func validEmoji(value string) bool { return emojiValues[value] }

type UpdateTeam struct {
	Key   string  `json:"key" jsonschema:"existing team key; key and numbering cannot change"`
	Emoji *string `json:"emoji,omitempty" jsonschema:"Unicode emoji icon; omitted leaves unchanged, empty clears"`
}

func (s *Service) UpdateTeam(ctx context.Context, input UpdateTeam) (Team, error) {
	if input.Emoji != nil && !validEmoji(*input.Emoji) {
		return Team{}, ErrInvalidEmoji
	}
	var team Team
	err := s.db.QueryRowContext(ctx, `UPDATE teams SET emoji = COALESCE(?, emoji) WHERE workspace_id = ? AND key = ? RETURNING key, name, next_number, emoji`, input.Emoji, s.workspace, input.Key).Scan(&team.Key, &team.Name, &team.NextNumber, &team.Emoji)
	if errors.Is(err, sql.ErrNoRows) {
		return Team{}, ErrTeamNotFound
	}
	return team, err
}
