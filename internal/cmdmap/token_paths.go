package cmdmap

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

type TokenPath struct {
	Kind        string   `json:"kind"`
	Tokens      []string `json:"tokens"`
	ValueToken  string   `json:"value_token,omitempty"`
	Description string   `json:"description"`
}

//go:embed command_schema.json
var commandSchemaJSON []byte

var TokenPaths = mustLoadTokenPaths()

func mustLoadTokenPaths() []TokenPath {
	var out []TokenPath
	if err := json.Unmarshal(commandSchemaJSON, &out); err != nil {
		panic(fmt.Sprintf("load command schema: %v", err))
	}
	return out
}
