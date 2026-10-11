package mcpserver

import (
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// addTool keeps output schemas open for clients that cache tools/list. Prepare
// the schema before AddTool resolves it so server and client validation agree.
// Input inference and validation remain the SDK's responsibility.
func addTool[In, Out any](server *mcp.Server, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	schema, err := jsonschema.For[Out](nil)
	if err != nil {
		panic(err)
	}
	allowAdditionalOutputProperties(schema)
	tool.OutputSchema = schema
	mcp.AddTool(server, tool, handler)
}

func allowAdditionalOutputProperties(schema *jsonschema.Schema) {
	if schema == nil {
		return
	}
	// jsonschema-go represents false as {"not":{}}. Keep typed map-value
	// schemas intact; only remove the blanket rejection inferred for structs.
	if reflect.DeepEqual(schema.AdditionalProperties, &jsonschema.Schema{Not: &jsonschema.Schema{}}) {
		schema.AdditionalProperties = nil
	}
	// Visit schema-valued fields, including properties, items, $defs and
	// composition keywords, without treating defaults or examples as schemas.
	value := reflect.ValueOf(schema).Elem()
	for i := range value.NumField() {
		switch child := value.Field(i).Interface().(type) {
		case *jsonschema.Schema:
			allowAdditionalOutputProperties(child)
		case []*jsonschema.Schema:
			for _, item := range child {
				allowAdditionalOutputProperties(item)
			}
		case map[string]*jsonschema.Schema:
			for _, item := range child {
				allowAdditionalOutputProperties(item)
			}
		}
	}
}
