// Package transforms provides message transformation middlewares for the LLM pipeline.
// These transforms operate on llm.Request.Messages and are designed to work with
// both Chat Completions API and Responses API paths.
package transforms

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

// TransformUserToToolResponse converts follow-up user messages (after assistant responses)
// into tool call/response pairs. This tricks the backend into treating user follow-ups
// as tool results.
func TransformUserToToolResponse(req *llm.Request) error {
	if req == nil {
		return errors.New("request is nil")
	}

	if len(req.Messages) == 0 {
		return nil
	}

	// Find the last assistant message without tool_calls or tool_use content
	// that has follow-up user messages to transform.
	targetAssistant := findTransformableAssistant(req.Messages)
	if targetAssistant == nil {
		return nil
	}

	for i := targetAssistant.index + 1; i < len(req.Messages); i++ {
		msg := &req.Messages[i]

		if msg.Role != "user" {
			continue
		}

		if hasToolResultContent(msg.Content) {
			continue
		}

		callID, err := generateCallID()
		if err != nil {
			return err
		}

		targetAssistant.msg.ToolCalls = append(targetAssistant.msg.ToolCalls, llm.ToolCall{
			ID:   callID,
			Type: "function",
			Function: llm.FunctionCall{
				Name:      "helper_temp_xxx_callback",
				Arguments: "{}",
			},
		})

		msg.Role = "tool"
		msg.ToolCallID = &callID
		transformUserContent(msg)
	}

	return nil
}

type assistantRef struct {
	index int
	msg   *llm.Message
}

func findTransformableAssistant(messages []llm.Message) *assistantRef {
	var lastAssistant *assistantRef

	for i := range messages {
		msg := &messages[i]
		if msg.Role != "assistant" {
			continue
		}

		if len(msg.ToolCalls) > 0 {
			continue
		}

		if hasToolUseContent(msg.Content) {
			continue
		}

		lastAssistant = &assistantRef{index: i, msg: msg}
	}

	return lastAssistant
}

func hasToolUseContent(content llm.MessageContent) bool {
	for _, part := range content.MultipleContent {
		if part.Type == "tool_use" {
			return true
		}
	}
	return false
}

func hasToolResultContent(content llm.MessageContent) bool {
	for _, part := range content.MultipleContent {
		if part.Type == "tool_result" {
			return true
		}
	}
	return false
}

func transformUserContent(msg *llm.Message) {
	content := msg.Content

	if content.Content != nil {
		return
	}

	if len(content.MultipleContent) == 0 {
		return
	}

	allText := true
	for _, part := range content.MultipleContent {
		if part.Type != "text" {
			allText = false
			break
		}
	}

	if allText {
		var flattened string
		for i, part := range content.MultipleContent {
			if part.Text != nil {
				if i > 0 {
					flattened += "\n"
				}
				flattened += "User Input：" + *part.Text
			}
		}
		msg.Content = llm.MessageContent{Content: &flattened}
	}
}

func generateCallID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "call_" + hex.EncodeToString(bytes), nil
}

// NewTransformUserToToolResponseMiddleware creates a pipeline middleware that
// transforms user messages after assistant to tool responses.
func NewTransformUserToToolResponseMiddleware() pipeline.Middleware {
	return pipeline.OnLlmRequest("transform-user-to-tool-response", func(ctx context.Context, request *llm.Request) (*llm.Request, error) {
		if err := TransformUserToToolResponse(request); err != nil {
			return nil, err
		}
		return request, nil
	})
}
