package mcpserver

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"testing"
)

func TestEntityEmojiMCP(t *testing.T) {
	session := connect(t)
	var result map[string]any
	call(t, session, "create_project", map[string]any{"teams": []string{"AL"}, "name": "Core"}, &result)
	for _, tc := range []struct{ tool, key, value string }{{"update_project", "name", "Core"}, {"update_team", "key", "AL"}} {
		for _, emoji := range []string{"🇨🇳", "👩🏽‍💻", ""} {
			call(t, session, tc.tool, map[string]any{tc.key: tc.value, "emoji": emoji}, &result)
			if result["emoji"] != emoji {
				t.Fatalf("%s emoji = %#v", tc.tool, result["emoji"])
			}
		}
		for _, args := range []map[string]any{{tc.key: tc.value, "emoji": "abc"}, {tc.key: "missing", "emoji": "🚀"}} {
			r, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: args})
			if err == nil && !r.IsError {
				t.Fatalf("%s accepted %#v", tc.tool, args)
			}
		}
	}
}
