// Package tools defines LLM-callable functions and dispatches their local implementations.
package tools

import (
	"encoding/json"
	"log"

	"simple-ai-service/internal/logging"

	openai "github.com/sashabaranov/go-openai"
)

// getWeatherTool is the schema the model sees. It doesn't run any code itself;
// it just tells the LLM "here's a function you can ask me to call,
// and here's what arguments it needs."
var getWeatherTool = openai.Tool{
	Type: openai.ToolTypeFunction,
	Function: &openai.FunctionDefinition{
		Name:        "get_weather",
		Description: "Get the current weather for a given city",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"location": {
					"type": "string",
					"description": "City name, e.g. Tokyo"
				}
			},
			"required": ["location"]
		}`),
	},
}

// All returns tool schemas offered to the model on chat completion requests.
func All() []openai.Tool {
	return []openai.Tool{getWeatherTool}
}

// Names returns a short list of registered tool names for logging.
func Names() []string {
	return []string{"get_weather"}
}

// getWeather is the Go implementation behind the tool. In a real service this
// would call a weather API; here it returns a fixed value so the example has
// zero external dependencies besides the LLM provider.
func getWeather(location string) string {
	result := `{"location":"` + location + `","forecast":"sunny","tempC":24}`
	log.Printf("[tool] get_weather(location=%q) → %s", location, result)
	return result
}

// Call dispatches a tool name to its Go implementation.
func Call(name, argsJSON string) string {
	log.Printf("[tool] dispatch name=%q args=%s", name, logging.Truncate(argsJSON, 200))
	switch name {
	case "get_weather":
		var args struct {
			Location string `json:"location"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			log.Printf("[tool] failed to parse args for %q: %v", name, err)
			return `{"error":"invalid arguments"}`
		}
		return getWeather(args.Location)
	default:
		log.Printf("[tool] unknown tool %q — returning error payload", name)
		return `{"error":"unknown tool"}`
	}
}
