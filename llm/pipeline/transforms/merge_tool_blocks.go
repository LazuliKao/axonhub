// Package transforms provides message transformation middlewares for the LLM pipeline.
// These transforms operate on llm.Request.Messages and are designed to work with
// both Chat Completions API and Responses API paths.
package transforms

import (
	"context"
	"errors"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/samber/lo"
)

const toolResultMergePrefix = "\n\nPlease execute skill now:"

// MergeToolResultBlocks merges tool messages with adjacent user messages.
// In the unified model, tool results are role: "tool" messages.
//
// The transform:
// 1. Iterates through messages
// 2. When a role: "tool" message is found, checks if the NEXT message is role: "user"
// 3. If yes, merges the user's text content INTO the tool message's content
// 4. Removes the user message from the array
// 5. The merged content format: {tool_text}\n\nPlease execute skill now:{user_text}
//
// Only merges ONE user message immediately following a tool message.
// Does NOT modify assistant messages.
func MergeToolResultBlocks(req *llm.Request) error {
	if req == nil {
		return errors.New("request is nil")
	}

	if len(req.Messages) == 0 {
		return nil
	}

	result := make([]llm.Message, 0, len(req.Messages))
	i := 0

	for i < len(req.Messages) {
		msg := req.Messages[i]

		// If this is a tool message and the next message is a user message, merge them
		if msg.Role == "tool" && i+1 < len(req.Messages) && req.Messages[i+1].Role == "user" {
			userMsg := req.Messages[i+1]
			merged := mergeToolWithUser(msg, userMsg)
			result = append(result, merged)
			i += 2 // Skip both tool and user messages
			continue
		}

		result = append(result, msg)
		i++
	}

	req.Messages = result
	return nil
}

// mergeToolWithUser merges a tool message with a following user message.
// The user's text content is appended to the tool's content with the merge prefix.
func mergeToolWithUser(tool, user llm.Message) llm.Message {
	result := tool

	userText := extractTextContent(user)
	if userText == "" {
		return result
	}

	toolText := extractTextContent(tool)

	var sb strings.Builder
	sb.WriteString(toolText)
	sb.WriteString(toolResultMergePrefix)
	sb.WriteString(userText)

	result.Content = llm.MessageContent{Content: lo.ToPtr(sb.String())}
	return result
}

// extractTextContent extracts all text content from a message.
// For string content, returns the string directly.
// For array content, concatenates all text parts.
func extractTextContent(msg llm.Message) string {
	if msg.Content.Content != nil {
		return *msg.Content.Content
	}

	var texts []string
	for _, part := range msg.Content.MultipleContent {
		if part.Type == "text" && part.Text != nil {
			texts = append(texts, *part.Text)
		}
	}
	return strings.Join(texts, "")
}

// NewMergeToolResultBlocksMiddleware creates a pipeline middleware that
// merges tool messages with adjacent user messages.
func NewMergeToolResultBlocksMiddleware() pipeline.Middleware {
	return pipeline.OnLlmRequest("merge-tool-result-blocks", func(ctx context.Context, request *llm.Request) (*llm.Request, error) {
		if err := MergeToolResultBlocks(request); err != nil {
			return nil, err
		}
		return request, nil
	})
}
