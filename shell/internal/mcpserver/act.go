package mcpserver

type actInput struct {
	Action string  `json:"action" jsonschema:"run or kill (required)."`
	Data   actData `json:"data,omitempty" jsonschema:"Options for the chosen action."`
}

type actData struct {
	Command    string   `json:"command,omitempty" jsonschema:"Command line (run)."`
	Cwd        string   `json:"cwd,omitempty" jsonschema:"Working directory for the command (run)."`
	Env        []string `json:"env,omitempty" jsonschema:"Extra env vars as KEY=VALUE (run)."`
	Background bool     `json:"background,omitempty" jsonschema:"run: start in background and return an id (logs via shell_read)."`
	Timeout    int      `json:"timeout,omitempty" jsonschema:"Foreground timeout in seconds (default SHELL_TIMEOUT_SECONDS)."`
	ID         string   `json:"id,omitempty" jsonschema:"Process id (kill)."`
}
