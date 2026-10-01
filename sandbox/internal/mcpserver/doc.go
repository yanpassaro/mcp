package mcpserver

import (
	"context"
	"fmt"
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
	"fetch":     "net",
	"http":      "net",
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
	if isIndexTopic(topic) {
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
	return textResult(fmt.Sprintf("%s\n\n⚠️ Topic `%s` not found. Available topics: %s.\n", renderLunaIndex(), strings.TrimSpace(in.Topic), strings.Join(lunaTopicNames(), ", ")))
}

func isIndexTopic(topic string) bool {
	if topic == "" {
		return true
	}
	if topic == "all" {
		return true
	}
	if topic == "overview" {
		return true
	}
	if topic == "help" {
		return true
	}
	return topic == "index"
}
