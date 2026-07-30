# simple-ai-service

The smallest useful example of:
1. a Go REST service (`net/http`, one endpoint)
2. an AI tool definition (`get_weather`, a plain JSON schema)
3. using the OpenAI SDK to prompt the LLM and let it call that tool

## How it works

`POST /chat` takes `{"message": "..."}`, sends it to the model along with
the `get_weather` tool schema. If the model decides it needs the tool, the
server runs the matching Go function, feeds the result back to the model,
and returns the model's final natural-language answer as `{"reply": "..."}`.

No database, no framework, no extra tools — just the request/response loop
that every tool-using AI service is built from.

## Run it

```bash
go mod tidy          # downloads github.com/sashabaranov/go-openai
export OPENAI_API_KEY=sk-...
go run main.go
```

## Try it

```bash
curl -X POST localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"What is the weather in Tokyo?"}'
```

Expected: the model calls `get_weather`, gets back a fixed "sunny, 24°C"
result, and replies in plain English mentioning that.

## Extending it

- Add more tools: define another `openai.Tool` + case in `callTool`.
- Swap the fake `getWeather` for a real API call.
- Swap `sashabaranov/go-openai` for the official `openai/openai-go` SDK if
  you prefer — the tool-calling shape (`tools`, `tool_calls`, `tool` role
  message) is the same across SDKs since it mirrors OpenAI's API directly.