package app

import (
	_ "embed"
	"encoding/json"
)

// Unicode 16.0 fully-qualified sequences, shared with the web picker.
// Source: https://unicode.org/Public/emoji/16.0/emoji-test.txt
// The Unicode data license is in emojidata/LICENSE.
//
//go:embed emojidata/emoji.json
var emojiData []byte

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
