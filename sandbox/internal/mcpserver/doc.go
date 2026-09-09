package mcpserver

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type docInput struct {
	Topic string `json:"topic,omitempty" jsonschema:"Documentation topic (module or section). If omitted, returns the index/overview."`
}

var docAliases = map[string]string{
	"":         "index",
	"all":      "index",
	"overview": "index",
	"help":     "index",
	"fs":       "io",
	"file":     "io",
	"files":    "io",
	"enc":      "encode",
	"strings":  "str",
	"text":     "str",
	"arrays":   "list",
	"array":    "list",
	"numbers":  "num",
	"number":   "num",
	"números":  "num",
	"dates":    "date",
	"date":     "date",
	"cookies":   "cookies",
	"sqlite":    "sql",
	"csv":       "csv",
	"regex":     "regex",
	"regexp":    "regex",
	"fake":      "fake",
	"faker":     "fake",
	"xml":       "xml",
	"excel":     "excel",
	"data":      "data",
	"pipeline":  "data",
	"xlsx":      "excel",
	"spreadsheet": "excel",
	"examples":  "examples",
	"cookbook":  "examples",
	"recipes":   "examples",
	"limits":    "limits",
	"env":      "env",
	"meta":     "meta",
	"tools":    "tools",
	"run":      "run",
	"runner":   "run",
	"execute":  "run",
	"exec":     "run",
	"executar": "run",
}

func (s *Server) doc(ctx context.Context, _ *mcp.CallToolRequest, in docInput) (*mcp.CallToolResult, any, error) {
	topic := strings.ToLower(strings.TrimSpace(in.Topic))
	if topic == "" || topic == "all" || topic == "overview" || topic == "help" || topic == "index" {
		return textResult(renderLunaIndex())
	}
	if alias, ok := docAliases[topic]; ok {
		topic = alias
	}
	if mod := lunaModule(topic); mod != nil {
		return textResult(renderLunaModule(mod))
	}
	if mod := lunaMetaTopic(topic); mod != nil {
		return textResult(renderLunaModule(mod))
	}
	return textResult(renderLunaIndex() + "\n\n⚠️ Topic `" + strings.TrimSpace(in.Topic) + "` not found. Available topics: " + strings.Join(lunaTopicNames(), ", ") + ".\n")
}
