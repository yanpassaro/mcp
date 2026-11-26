package mcpserver

type input struct {
	Action string `json:"action" jsonschema:"list, log or allow (required)."`
	Data   data   `json:"data,omitempty" jsonschema:"Options for the chosen action."`
}

type data struct {
	ID  string `json:"id,omitempty" jsonschema:"Process id (log)."`
	Max int    `json:"max,omitempty" jsonschema:"Log: bytes from the end (default 8192)."`
}
