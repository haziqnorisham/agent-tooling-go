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
go mod tidy                        # downloads dependencies
cp .env.example .env               # optional — edit with your values
# or: export OPENAI_API_KEY=sk-...
go run main.go
```

On startup the service loads a local `.env` file if present (via `godotenv`),
then reads these variables. Values already set in the process environment
always win over `.env`.

| Variable | Default | Description |
|---|---|---|
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | Base URL of any OpenAI-compatible API |
| `OPENAI_API_KEY` | — | API key (empty is fine for local servers) |
| `OPENAI_MODEL` | `gpt-4o-mini` | Model name to use for chat completions |

## Try it

```bash
curl -X POST localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"What is the weather in Tokyo?"}'
```

Expected: the model calls `get_weather`, gets back a fixed "sunny, 24°C"
result, and replies in plain English mentioning that.

## Pointing at another OpenAI-compatible provider

The service works against any server that implements the OpenAI chat
completions API — Ollama, LM Studio, vLLM, OpenRouter, Azure (via an
OpenAI-compatible gateway), etc.

### Ollama (local)

```bash
# First pull a model, e.g.:
#   ollama pull llama3.2

export OPENAI_BASE_URL=http://localhost:11434/v1
export OPENAI_API_KEY=ollama       # Ollama accepts any non-empty value
export OPENAI_MODEL=llama3.2
go run main.go
```

### OpenRouter

```bash
export OPENAI_BASE_URL=https://openrouter.ai/api/v1
export OPENAI_API_KEY=sk-or-...    # from https://openrouter.ai/keys
export OPENAI_MODEL=openai/gpt-4o-mini
go run main.go
```

**Note:** tool-calling support depends on the target provider and model.
Some servers ignore the `tools` field entirely — in that case the service
still works for basic chat, but the tool loop is skipped.

## Extending it

- Add more tools: define another `openai.Tool` + case in `callTool`.
- Swap the fake `getWeather` for a real API call.
- Swap `sashabaranov/go-openai` for the official `openai/openai-go` SDK if
  you prefer — the tool-calling shape (`tools`, `tool_calls`, `tool` role
  message) is the same across SDKs since it mirrors OpenAI's API directly.