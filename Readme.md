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

No database, no framework — just the request/response loop that every
tool-using AI service is built from.

Logs go to **stdout** with step tags (`[config]`, `[http]`, `[chat]`,
`[llm]`, `[tool]`, `[flow]`) so you can watch config load, each LLM round,
tool dispatch, and the final reply without a debugger.

## Layout

```
cmd/server/main.go          # process entrypoint (wire + start)
internal/config/            # env / .env loading
internal/logging/           # stdout log setup + helpers
internal/tools/             # tool schemas + local implementations
internal/llm/               # OpenAI-compatible client + tool-calling loop
internal/handler/           # HTTP request/response for /chat
internal/server/            # ServeMux + ListenAndServe
```

## Run it

```bash
go mod tidy                        # downloads dependencies
cp .env.example .env               # optional — edit with your values
# or: export OPENAI_API_KEY=sk-...
go run ./cmd/server
```

On startup the service loads a local `.env` file if present (via `godotenv`),
then reads these variables. Values already set in the process environment
always win over `.env`.

| Variable | Default | Description |
|---|---|---|
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | Base URL of any OpenAI-compatible API |
| `OPENAI_API_KEY` | — | API key (empty is fine for local servers) |
| `OPENAI_MODEL` | `gpt-4o-mini` | Model name to use for chat completions |
| `HTTP_ADDR` | `:8080` | HTTP listen address |

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
go run ./cmd/server
```

### OpenRouter

```bash
export OPENAI_BASE_URL=https://openrouter.ai/api/v1
export OPENAI_API_KEY=sk-or-...    # from https://openrouter.ai/keys
export OPENAI_MODEL=openai/gpt-4o-mini
go run ./cmd/server
```

**Note:** tool-calling support depends on the target provider and model.
Some servers ignore the `tools` field entirely — in that case the service
still works for basic chat, but the tool loop is skipped.

## Extending it

- Add more tools: define another schema + case in `internal/tools`.
- Swap the fake `getWeather` for a real API call.
- Swap `sashabaranov/go-openai` for the official `openai/openai-go` SDK if
  you prefer — the tool-calling shape (`tools`, `tool_calls`, `tool` role
  message) is the same across SDKs since it mirrors OpenAI's API directly.

## Production hardening

This boilerplate is minimal by design. The following are recommendations
in order of priority for taking it to production.

### Must-haves

**Tool-loop safety.** Three things people skip and regret:

1. **Max iteration count** — models can loop forever. Cap the tool-calling
   loop (e.g. 5 rounds), then force a final answer or error out.

2. **Per-request timeout** — thread a `context.WithTimeout` through every
   LLM and tool call. A single runaway request blocks the server.

3. **Duplicate call guard** — the model sometimes calls the same tool with
   the same arguments multiple times. Hash the call (name + args) and skip
   or bail if the hash is seen twice in one request.

**Chat history persistence.** You don't need a heavy DB for this. Options
in order of simplicity:

- **SQLite** (via `mattn/go-sqlite3` or `modernc.org/sqlite` for pure-Go,
  no cgo) — genuinely enough for most single-instance deployments.
- **Postgres** only once you need multiple app instances sharing state.

Store: `conversation_id`, `role`, `content`, `tool_calls`/`tool_results`
as JSON, `created_at`. Load history by `conversation_id`, cap it (last N
messages or a token budget) before sending to the model — unbounded
history will blow your context window and your bill.

**Streaming endpoint.** Add a separate `/chat/stream` using SSE
(`text/event-stream`). The OpenAI-compatible streaming API sends delta
chunks; forward each delta to the client as it arrives. Tool calls
complicate this — the model streams tool-call arguments as fragments too,
so you buffer until the call is complete, execute it, then resume
streaming.

**Tool argument validation.** Right now `json.Unmarshal` silently accepts
garbage. Validate against the JSON schema (or at minimum check required
fields aren't empty) before executing a tool, and return a structured
error back to the model rather than crashing or passing bad data through.

**Retry/backoff on LLM calls.** Rate limits and transient 5xxs are
routine. Wrap the OpenAI call with a small retry (2-3 attempts,
exponential backoff) for retryable status codes only.

**Graceful shutdown.** `http.Server` with `Shutdown(ctx)` on `SIGTERM` so
in-flight requests (especially streaming ones) finish cleanly instead of
getting cut.

### Should-haves

**Tool registry pattern.** Instead of a growing `switch` in `callTool`,
use a `map[string]Tool` where each tool is `{Schema, Handler}`. Makes
adding tools additive, not an edit to a central function. Worth doing
before you add more than ~3 tools.

**Structured logging with request IDs.** You already have stdout logging;
the next step is a request/conversation ID threaded through every log
line so you can trace one conversation's full tool-call chain in
production.

**Basic auth on the endpoint.** Even a static API key header check. It's
your service, not OpenAI's — nothing stops randos from hitting it and
spending your OpenAI budget if it's public.

**Token/cost guardrails.** Cap `max_tokens` per response, and consider a
per-conversation or per-day spend ceiling if this is user-facing.

### Probably skip for now

These add complexity disproportionate to value at this stage:

- **Vector DB / RAG** — only if you actually need retrieval; don't add
  speculatively.
- **Multi-agent orchestration frameworks** — a single tool loop is fine
  until you have a concrete reason for more.
- **Kubernetes / complex deployment** — a Dockerfile + single container
  is enough until you have real scale needs.
- **Websockets over SSE** for streaming — SSE is simpler and sufficient
  unless you need bidirectional push.
