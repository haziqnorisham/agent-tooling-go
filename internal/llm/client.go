// Package llm wraps an OpenAI-compatible chat client and the tool-calling loop.
package llm

import (
	"context"
	"fmt"
	"log"
	"time"

	"simple-ai-service/internal/logging"
	"simple-ai-service/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

// Client talks to any OpenAI-compatible chat completions API.
type Client struct {
	api   *openai.Client
	model string
}

// New builds a client pointed at cfgBaseURL with the given API key and model.
func New(baseURL, apiKey, model string) *Client {
	ocfg := openai.DefaultConfig(apiKey)
	ocfg.BaseURL = baseURL
	return &Client{
		api:   openai.NewClientWithConfig(ocfg),
		model: model,
	}
}

// Model returns the configured model name.
func (c *Client) Model() string {
	return c.model
}

// Chat sends the user message, optionally runs tools the model requested,
// and returns the final natural-language reply.
//
// Tool-calling support depends on the target provider and model — some
// OpenAI-compatible servers ignore the tools field entirely. In that case
// the model returns a plain text reply in the first turn and the tool loop
// is skipped, so the service still works for basic chat.
func (c *Client) Chat(ctx context.Context, userMessage string) (string, error) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: userMessage},
	}

	log.Printf("[llm] round 1 → CreateChatCompletion model=%s tools=%v messages=1", c.model, tools.Names())
	t1 := time.Now()
	resp, err := c.api.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: messages,
		Tools:    tools.All(),
	})
	if err != nil {
		log.Printf("[llm] round 1 error after %s: %v", time.Since(t1).Round(time.Millisecond), err)
		return "", fmt.Errorf("chat completion round 1: %w", err)
	}
	log.Printf("[llm] round 1 ← done in %s id=%s choices=%d",
		time.Since(t1).Round(time.Millisecond), resp.ID, len(resp.Choices))

	if len(resp.Choices) == 0 {
		log.Printf("[llm] round 1 returned no choices")
		return "", fmt.Errorf("model returned no choices")
	}

	msg := resp.Choices[0].Message
	finish := resp.Choices[0].FinishReason
	log.Printf("[llm] round 1 finish_reason=%q content=%q tool_calls=%d",
		finish, logging.Truncate(msg.Content, 200), len(msg.ToolCalls))

	if len(msg.ToolCalls) == 0 {
		log.Printf("[flow] no tool calls — using first-round content as the final reply")
		return msg.Content, nil
	}

	log.Printf("[flow] model requested %d tool call(s) — running local implementations", len(msg.ToolCalls))
	messages = append(messages, msg)

	for i, tc := range msg.ToolCalls {
		log.Printf("[flow] tool_call[%d] id=%s name=%s", i, tc.ID, tc.Function.Name)
		result := tools.Call(tc.Function.Name, tc.Function.Arguments)
		messages = append(messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    result,
			ToolCallID: tc.ID,
		})
		log.Printf("[flow] appended tool role message for tool_call_id=%s", tc.ID)
	}

	log.Printf("[llm] round 2 → CreateChatCompletion model=%s tools=none messages=%d (user + assistant + tool results)",
		c.model, len(messages))
	t2 := time.Now()
	resp, err = c.api.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: messages,
	})
	if err != nil {
		log.Printf("[llm] round 2 error after %s: %v", time.Since(t2).Round(time.Millisecond), err)
		return "", fmt.Errorf("chat completion round 2: %w", err)
	}
	if len(resp.Choices) == 0 {
		log.Printf("[llm] round 2 returned no choices")
		return "", fmt.Errorf("model returned no choices")
	}
	msg = resp.Choices[0].Message
	log.Printf("[llm] round 2 ← done in %s id=%s finish_reason=%q content=%q",
		time.Since(t2).Round(time.Millisecond),
		resp.ID,
		resp.Choices[0].FinishReason,
		logging.Truncate(msg.Content, 300),
	)
	return msg.Content, nil
}
