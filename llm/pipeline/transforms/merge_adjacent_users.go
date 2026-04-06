// Package transforms provides message transformation middlewares for the LLM pipeline.
// These transforms operate on llm.Request.Messages and are designed to work with
// both Chat Completions API and Responses API paths.
package transforms

import (
	"context"
	"errors"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

// MergeAdjacentUserMessages merges consecutive user messages in the request into a single message.
// This reduces message count and request complexity.
//
// Merging rules:
//   - string + string → join with "\n\n"
//   - string + array → convert to array with both as text parts
//   - array + string → append string as text part
//   - array + array → concatenate arrays
//   - empty string on either side → drop empty, keep non-empty
//
// Only user messages are merged. Messages separated by other roles are not merged.
func MergeAdjacentUserMessages(req *llm.Request) error {
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

		if msg.Role != "user" {
			result = append(result, msg)
			i++
			continue
		}

		// Collect consecutive user messages
		merged := msg
		j := i + 1

		for j < len(req.Messages) && req.Messages[j].Role == "user" {
			merged = mergeUserMessages(merged, req.Messages[j])
			j++
		}

		result = append(result, merged)
		i = j
	}

	req.Messages = result
	return nil
}

// mergeUserMessages merges two consecutive user messages into one.
func mergeUserMessages(first, second llm.Message) llm.Message {
	result := first

	firstContent := first.Content
	secondContent := second.Content

	// Determine the merged content type
	switch {
	case isEmptyContent(firstContent) && isEmptyContent(secondContent):
		// Both empty, keep first
		result.Content = firstContent

	case isEmptyContent(firstContent):
		// First empty, keep second
		result.Content = secondContent

	case isEmptyContent(secondContent):
		// Second empty, keep first
		result.Content = firstContent

	case firstContent.Content != nil && secondContent.Content != nil:
		// Both string content → join with \n\n
		merged := *firstContent.Content + "\n\n" + *secondContent.Content
		result.Content = llm.MessageContent{Content: &merged}

	case firstContent.Content != nil && len(secondContent.MultipleContent) > 0:
		// First string + second array → convert to array
		parts := make([]llm.MessageContentPart, 0, 1+len(secondContent.MultipleContent))
		parts = append(parts, llm.MessageContentPart{Type: "text", Text: firstContent.Content})
		parts = append(parts, secondContent.MultipleContent...)
		result.Content = llm.MessageContent{MultipleContent: parts}

	case len(firstContent.MultipleContent) > 0 && secondContent.Content != nil:
		// First array + second string → append as text part
		parts := make([]llm.MessageContentPart, 0, len(firstContent.MultipleContent)+1)
		parts = append(parts, firstContent.MultipleContent...)
		parts = append(parts, llm.MessageContentPart{Type: "text", Text: secondContent.Content})
		result.Content = llm.MessageContent{MultipleContent: parts}

	case len(firstContent.MultipleContent) > 0 && len(secondContent.MultipleContent) > 0:
		// Both arrays → concatenate
		parts := make([]llm.MessageContentPart, 0, len(firstContent.MultipleContent)+len(secondContent.MultipleContent))
		parts = append(parts, firstContent.MultipleContent...)
		parts = append(parts, secondContent.MultipleContent...)
		result.Content = llm.MessageContent{MultipleContent: parts}
	}

	return result
}

// isEmptyContent checks if the content is empty or nil.
func isEmptyContent(content llm.MessageContent) bool {
	if content.Content != nil && *content.Content != "" {
		return false
	}
	if len(content.MultipleContent) > 0 {
		return false
	}
	return true
}

// NewMergeAdjacentUserMessagesMiddleware creates a pipeline middleware that
// merges consecutive user messages in the request.
func NewMergeAdjacentUserMessagesMiddleware() pipeline.Middleware {
	return pipeline.OnLlmRequest("merge-adjacent-user-messages", func(ctx context.Context, request *llm.Request) (*llm.Request, error) {
		if err := MergeAdjacentUserMessages(request); err != nil {
			return nil, err
		}
		return request, nil
	})
}
