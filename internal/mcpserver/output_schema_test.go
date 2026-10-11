package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func listedTools(t *testing.T, session *mcp.ClientSession) map[string]*mcp.Tool {
	t.Helper()
	tools := make(map[string]*mcp.Tool)
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		tools[tool.Name] = tool
	}
	if len(tools) == 0 {
		t.Fatal("tools/list returned no tools")
	}
	return tools
}

func TestToolOutputSchemasAllowAdditionalProperties(t *testing.T) {
	tools := listedTools(t, connect(t))
	for _, name := range []string{"get_issue", "list_issues", "list_labels", "merge_labels"} {
		if tools[name] == nil {
			t.Fatalf("tools/list missing %s", name)
		}
	}
	for name, tool := range tools {
		t.Run(name, func(t *testing.T) {
			if tool.OutputSchema == nil {
				t.Fatal("outputSchema is missing")
			}
			var schema any
			decodeSchema(t, tool.OutputSchema, &schema)
			assertOpenOutputObjects(t, "$", schema)
		})
	}
}

// Inspect the wire representation independently of the production schema walker.
func assertOpenOutputObjects(t *testing.T, path string, value any) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		// jsonschema-go represents the boolean false schema as {"not":{}}.
		additional := value["additionalProperties"]
		if additional == false || reflect.DeepEqual(additional, map[string]any{"not": map[string]any{}}) {
			t.Errorf("%s forbids additional properties", path)
		}
		for key, child := range value {
			assertOpenOutputObjects(t, path+"/"+key, child)
		}
	case []any:
		for i, child := range value {
			assertOpenOutputObjects(t, fmt.Sprintf("%s/%d", path, i), child)
		}
	}
}

func decodeSchema(t *testing.T, value, target any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
}

func resolvedSchema(t *testing.T, value any) *jsonschema.Resolved {
	t.Helper()
	var schema jsonschema.Schema
	decodeSchema(t, value, &schema)
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestIssueOutputSchemasAcceptAdditionalFields(t *testing.T) {
	session := connect(t)
	// Cache the advertised schemas as a client would at connection time.
	tools := listedTools(t, session)
	var issue app.Issue
	var label app.Label
	var ignored struct{}
	call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Parent"}, &issue)
	call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Child", "parent": "AL-1"}, &issue)
	call(t, session, "create_label", map[string]any{"name": "Release", "color": "#123456"}, &label)
	call(t, session, "change_issue_labels", map[string]any{"identifiers": []string{"AL-1", "AL-2"}, "add": []int64{label.ID}}, &ignored)
	call(t, session, "add_comment", map[string]any{"identifier": "AL-1", "body": "Context"}, &ignored)
	call(t, session, "add_link", map[string]any{"identifier": "AL-1", "title": "Design", "url": "https://example.com"}, &ignored)

	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"get_issue", map[string]any{"identifier": "AL-1"}},
		{"list_issues", map[string]any{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema := resolvedSchema(t, tools[tc.name].OutputSchema)
			var original map[string]any
			call(t, session, tc.name, tc.args, &original)
			if err := schema.Validate(original); err != nil {
				t.Fatalf("real structuredContent: %v", err)
			}
			for _, location := range []string{"root", "nested"} {
				t.Run(location, func(t *testing.T) {
					var output map[string]any
					decodeSchema(t, original, &output)
					if location == "root" {
						output["future_field"] = "new server value"
					} else {
						for _, value := range output {
							addFutureFields(value)
						}
					}
					if err := schema.Validate(output); err != nil {
						t.Errorf("cached schema rejected additive output: %v", err)
					}
				})
			}
			for _, mutation := range []string{"missing required", "wrong type"} {
				t.Run(mutation, func(t *testing.T) {
					var output map[string]any
					decodeSchema(t, original, &output)
					object := output
					if tc.name == "list_issues" {
						object = output["issues"].([]any)[0].(map[string]any)
					}
					if mutation == "missing required" {
						delete(object, "identifier")
					} else {
						object["identifier"] = 42
					}
					if err := schema.Validate(output); err == nil {
						t.Fatal("schema accepted an incompatible output change")
					}
				})
			}
		})
	}
}

func addFutureFields(value any) {
	switch value := value.(type) {
	case map[string]any:
		for _, child := range value {
			addFutureFields(child)
		}
		value["future_field"] = "new server value"
	case []any:
		for _, child := range value {
			addFutureFields(child)
		}
	}
}

func TestToolInputsRejectAdditionalProperties(t *testing.T) {
	session := connect(t)
	tools := listedTools(t, session)
	var issue app.Issue
	call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Existing"}, &issue)
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"get_issue", map[string]any{"identifier": "AL-1"}},
		{"list_issues", map[string]any{}},
		{"create_issue", map[string]any{"team": "AL", "title": "Rejected"}},
		{"create_label", map[string]any{"name": "Rejected", "color": "#123456"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema := resolvedSchema(t, tools[tc.name].InputSchema)
			if err := schema.Validate(tc.args); err != nil {
				t.Fatalf("valid arguments: %v", err)
			}
			tc.args["unknown_argument"] = true
			if err := schema.Validate(tc.args); err == nil {
				t.Error("inputSchema accepted an unknown argument")
			}
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err != nil {
				t.Fatal(err)
			}
			content, err := json.Marshal(result.Content)
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError || !strings.Contains(string(content), "unexpected additional properties") {
				t.Fatalf("server did not reject unknown argument: %s", content)
			}
		})
	}
}

func TestOutputSchemaDefinitionsAllowAdditionalFields(t *testing.T) {
	var schema jsonschema.Schema
	if err := json.Unmarshal([]byte(`{
		"type":"object", "additionalProperties":false, "required":["records","lookup"],
		"properties":{
			"records":{"type":"array","items":{"$ref":"#/$defs/record"}},
			"lookup":{"type":"object","additionalProperties":{"$ref":"#/$defs/record"}}
		},
		"$defs":{"record":{
			"type":["object","null"], "additionalProperties":false, "required":["name","detail"],
			"properties":{
				"name":{"type":"string"},
				"detail":{"anyOf":[
					{"type":"null"},
					{"type":"object","additionalProperties":false,"required":["count"],"properties":{"count":{"type":"integer"}}}
				]}
			},
			"examples":[{"additionalProperties":false}]
		}}
	}`), &schema); err != nil {
		t.Fatal(err)
	}
	allowAdditionalOutputProperties(&schema)
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	record := map[string]any{"name": "item", "detail": map[string]any{"count": 1, "future_field": true}, "future_field": true}
	lookup := map[string]any{"key": record}
	output := map[string]any{"records": []any{record}, "lookup": lookup, "future_field": true}
	if err := resolved.Validate(output); err != nil {
		t.Fatalf("additive output through $defs, items, anyOf and map values: %v", err)
	}
	if !reflect.DeepEqual(schema.Defs["record"].Examples, []any{map[string]any{"additionalProperties": false}}) {
		t.Fatal("schema traversal modified example data")
	}
	lookup["key"] = 1
	if err := resolved.Validate(output); err == nil {
		t.Fatal("typed additionalProperties constraint was removed")
	}
	lookup["key"] = record
	delete(record, "name")
	if err := resolved.Validate(output); err == nil {
		t.Fatal("required field in $defs was removed")
	}
}

// Model output JSON that has evolved independently of its advertised Go type.
// The SDK validates marshaled JSON, including custom marshalers.
type schemaProbeOutput struct {
	Value string `json:"value"`
	raw   json.RawMessage
}

func (output schemaProbeOutput) MarshalJSON() ([]byte, error) {
	return output.raw, nil
}

func TestToolOutputValidation(t *testing.T) {
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "schema-test", Version: "1"}, nil)
	payload := json.RawMessage(`{"value":"ok","future_field":true}`)
	addTool(server, &mcp.Tool{Name: "probe"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, schemaProbeOutput, error) {
		return nil, schemaProbeOutput{raw: payload}, nil
	})
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "schema-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	advertised := resolvedSchema(t, listedTools(t, session)["probe"].OutputSchema)
	var output map[string]any
	call(t, session, "probe", map[string]any{}, &output)
	if output["future_field"] != true {
		t.Fatalf("server lost additive structured output: %v", output)
	}
	if err := advertised.Validate(output); err != nil {
		t.Fatalf("server and client schemas disagree: %v", err)
	}
	for _, invalid := range []string{`{}`, `{"value":42}`} {
		payload = json.RawMessage(invalid)
		_, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "probe", Arguments: map[string]any{}})
		if err == nil || !strings.Contains(err.Error(), "validating tool output") {
			t.Fatalf("server accepted invalid output %s: %v", invalid, err)
		}
	}
}
