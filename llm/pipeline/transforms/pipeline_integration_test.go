// Package transforms provides message transformation middlewares for the LLM pipeline.
package transforms

import (
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

// TestPipelineIntegration verifies all 3 transforms work correctly when chained together
// in the actual pipeline order: mergeAdjacentUserMessages → mergeToolResultBlocks → transformUserToToolResponse
func TestPipelineIntegration(t *testing.T) {
	tests := []struct {
		name string
		// initial messages to transform
		messages []llm.Message
		// expected number of messages after all transforms
		wantLen int
		// validation function for the final transformed messages
		wantContent func(t *testing.T, msgs []llm.Message)
	}{
		// Scenario 1: Messages Array Format - full pipeline with all 3 transforms
		// Initial: user, user, assistant, user
		// After mergeAdjacentUserMessages: user(merged), assistant, user
		// After mergeToolResultBlocks: no change (no tool messages)
		// After transformUserToToolResponse: user, assistant(with tool_calls), tool
		{
			name: "messages array format full pipeline",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("world")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("followup")}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				// First message: merged user messages
				assert.Equal(t, "user", msgs[0].Role)
				assert.Equal(t, "hello\n\nworld", *msgs[0].Content.Content)

				// Second message: assistant with tool_calls injected
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Len(t, msgs[1].ToolCalls, 1)
				assert.Equal(t, "function", msgs[1].ToolCalls[0].Type)
				assert.Equal(t, "helper_temp_xxx_callback", msgs[1].ToolCalls[0].Function.Name)
				assert.Contains(t, msgs[1].ToolCalls[0].ID, "call_")

				// Third message: user converted to tool
				assert.Equal(t, "tool", msgs[2].Role)
				assert.Equal(t, msgs[1].ToolCalls[0].ID, *msgs[2].ToolCallID)
				assert.Equal(t, "followup", *msgs[2].Content.Content)
			},
		},

		// Scenario 2: Complex scenario with all transform types
		// Tests interaction between mergeToolResultBlocks and transformUserToToolResponse
		// Initial: user, assistant, tool, user
		// After mergeAdjacentUserMessages: no change (no consecutive users)
		// After mergeToolResultBlocks: tool(merged with user)
		// After transformUserToToolResponse: no change (user was merged into tool)
		{
			name: "complex scenario with all transforms",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("initial")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool result")}, ToolCallID: lo.ToPtr("call_orig")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("followup after tool")}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				// First message: unchanged user
				assert.Equal(t, "user", msgs[0].Role)
				assert.Equal(t, "initial", *msgs[0].Content.Content)

				// Second message: unchanged assistant
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Equal(t, "response", *msgs[1].Content.Content)

				// Third message: tool merged with user
				assert.Equal(t, "tool", msgs[2].Role)
				assert.Contains(t, *msgs[2].Content.Content, "tool result")
				assert.Contains(t, *msgs[2].Content.Content, "Please execute skill now:")
				assert.Contains(t, *msgs[2].Content.Content, "followup after tool")
			},
		},

		// Scenario 3: Multiple consecutive users followed by assistant and user
		// Tests mergeAdjacentUserMessages + transformUserToToolResponse interaction
		{
			name: "multiple consecutive users then assistant then user",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("first")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("second")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("third")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("followup")}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				// First message: all three users merged
				assert.Equal(t, "user", msgs[0].Role)
				assert.Equal(t, "first\n\nsecond\n\nthird", *msgs[0].Content.Content)

				// Second message: assistant with tool_calls
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Len(t, msgs[1].ToolCalls, 1)

				// Third message: user converted to tool
				assert.Equal(t, "tool", msgs[2].Role)
				assert.Equal(t, "followup", *msgs[2].Content.Content)
			},
		},

		// Scenario 4: Tool message with multiple follow-up users
		// Tests pipeline order: mergeAdjacentUserMessages runs first, merging the two users
		// Then mergeToolResultBlocks merges the tool with the merged user
		{
			name: "tool message with multiple follow-up users",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("request")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}, ToolCalls: []llm.ToolCall{
					{ID: "call_1", Type: "function", Function: llm.FunctionCall{Name: "get_weather"}},
				}},
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("weather data")}, ToolCallID: lo.ToPtr("call_1")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("first followup")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("second followup")}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				// First message: user
				assert.Equal(t, "user", msgs[0].Role)
				assert.Equal(t, "request", *msgs[0].Content.Content)

				// Second message: assistant with original tool_calls
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Len(t, msgs[1].ToolCalls, 1)
				assert.Equal(t, "call_1", msgs[1].ToolCalls[0].ID)

				// Third message: tool merged with both users (they were merged first)
				assert.Equal(t, "tool", msgs[2].Role)
				assert.Contains(t, *msgs[2].Content.Content, "weather data")
				assert.Contains(t, *msgs[2].Content.Content, "first followup")
				assert.Contains(t, *msgs[2].Content.Content, "second followup")
			},
		},

		// Scenario 5: Empty messages - no-op
		{
			name:     "empty messages",
			messages: []llm.Message{},
			wantLen:  0,
		},

		// Scenario 6: Single user message - no transforms apply
		{
			name: "single user message",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user", msgs[0].Role)
				assert.Equal(t, "hello", *msgs[0].Content.Content)
			},
		},

		// Scenario 7: User with array content merged with user with string content
		{
			name: "user array content merged with user string content",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("array text")},
					},
				}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("string text")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				// First message: merged user with array content
				assert.Equal(t, "user", msgs[0].Role)
				assert.Len(t, msgs[0].Content.MultipleContent, 2)
				assert.Equal(t, "array text", *msgs[0].Content.MultipleContent[0].Text)
				assert.Equal(t, "string text", *msgs[0].Content.MultipleContent[1].Text)

				// Second message: assistant (no tool_calls because no follow-up user)
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Len(t, msgs[1].ToolCalls, 0)
			},
		},

		// Scenario 8: Full conversation with system message
		{
			name: "full conversation with system message",
			messages: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("You are helpful")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("world")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("greeting")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("question")}},
			},
			wantLen: 4,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "system", msgs[0].Role)
				assert.Equal(t, "user", msgs[1].Role)
				assert.Equal(t, "hello\n\nworld", *msgs[1].Content.Content)
				assert.Equal(t, "assistant", msgs[2].Role)
				assert.Len(t, msgs[2].ToolCalls, 1)
				assert.Equal(t, "tool", msgs[3].Role)
			},
		},

		// Scenario 9: Tool result with user having multiple text parts
		{
			name: "tool result with user having multiple text parts",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}, ToolCalls: []llm.ToolCall{
					{ID: "call_1", Type: "function", Function: llm.FunctionCall{Name: "tool"}},
				}},
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("result")}, ToolCallID: lo.ToPtr("call_1")},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("part one")},
						{Type: "text", Text: lo.ToPtr(" part two")},
					},
				}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "assistant", msgs[0].Role)
				assert.Equal(t, "tool", msgs[1].Role)
				// Tool content should have merged user text parts
				assert.Contains(t, *msgs[1].Content.Content, "result")
				assert.Contains(t, *msgs[1].Content.Content, "part one part two")
			},
		},

		// Scenario 10: Multiple assistant messages - only last transformable one gets tool_calls
		// findTransformableAssistant finds the LAST transformable assistant
		{
			name: "multiple assistant messages",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("first")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response1")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("second")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response2")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("third")}},
			},
			wantLen: 5,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				// First assistant should NOT have tool_calls (not the last transformable)
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Len(t, msgs[1].ToolCalls, 0)

				// Second assistant should have tool_calls (last transformable)
				assert.Equal(t, "assistant", msgs[3].Role)
				assert.Len(t, msgs[3].ToolCalls, 1)

				// Last user should be converted to tool
				assert.Equal(t, "tool", msgs[4].Role)

				// User "second" should remain as user
				assert.Equal(t, "user", msgs[2].Role)
			},
		},

		// Scenario 11: User with tool_result content should not be transformed
		{
			name: "user with tool_result content",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "tool_result", Text: lo.ToPtr("result")},
					},
				}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				// Assistant should NOT have tool_calls (user has tool_result)
				assert.Equal(t, "assistant", msgs[0].Role)
				assert.Len(t, msgs[0].ToolCalls, 0)

				// User should remain as user
				assert.Equal(t, "user", msgs[1].Role)
			},
		},

		// Scenario 12: Assistant with tool_use content should not transform following users
		{
			name: "assistant with tool_use content",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "tool_use", Text: lo.ToPtr("tool content")},
					},
				}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("followup")}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				// Assistant should NOT have tool_calls (has tool_use content)
				assert.Equal(t, "assistant", msgs[0].Role)
				assert.Len(t, msgs[0].ToolCalls, 0)

				// User should remain as user
				assert.Equal(t, "user", msgs[1].Role)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &llm.Request{Messages: tt.messages}

			// Apply all 3 transforms in pipeline order
			require.NoError(t, MergeAdjacentUserMessages(req))
			require.NoError(t, MergeToolResultBlocks(req))
			require.NoError(t, TransformUserToToolResponse(req))

			// Verify final state
			assert.Len(t, req.Messages, tt.wantLen)
			if tt.wantContent != nil {
				tt.wantContent(t, req.Messages)
			}
		})
	}
}

// TestPipelineTransformOrder verifies that transforms are applied in the correct order
// and that each transform's output is correctly consumed by the next transform.
func TestPipelineTransformOrder(t *testing.T) {
	t.Run("mergeAdjacentUserMessages runs before mergeToolResultBlocks", func(t *testing.T) {
		// If mergeToolResultBlocks ran first, it would see two users after tool
		// and only merge the first one. But mergeAdjacentUserMessages runs first,
		// merging the two users, so mergeToolResultBlocks sees only one user.
		messages := []llm.Message{
			{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("result")}, ToolCallID: lo.ToPtr("call_1")},
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("first")}},
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("second")}},
		}

		req := &llm.Request{Messages: messages}

		require.NoError(t, MergeAdjacentUserMessages(req))
		require.NoError(t, MergeToolResultBlocks(req))
		require.NoError(t, TransformUserToToolResponse(req))

		// Should have 1 message: tool merged with the merged user content
		require.Len(t, req.Messages, 1)
		assert.Equal(t, "tool", req.Messages[0].Role)
		assert.Contains(t, *req.Messages[0].Content.Content, "result")
		assert.Contains(t, *req.Messages[0].Content.Content, "first\n\nsecond")
	})

	t.Run("mergeToolResultBlocks runs before transformUserToToolResponse", func(t *testing.T) {
		// If transformUserToToolResponse ran first, it would convert the user to tool.
		// But mergeToolResultBlocks runs first, merging the user into the tool message.
		messages := []llm.Message{
			{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
			{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("result")}, ToolCallID: lo.ToPtr("call_1")},
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("followup")}},
		}

		req := &llm.Request{Messages: messages}

		require.NoError(t, MergeAdjacentUserMessages(req))
		require.NoError(t, MergeToolResultBlocks(req))
		require.NoError(t, TransformUserToToolResponse(req))

		// Should have 2 messages: assistant and merged tool
		require.Len(t, req.Messages, 2)
		assert.Equal(t, "assistant", req.Messages[0].Role)
		assert.Equal(t, "tool", req.Messages[1].Role)
		assert.Contains(t, *req.Messages[1].Content.Content, "result")
		assert.Contains(t, *req.Messages[1].Content.Content, "followup")
	})

	t.Run("transformUserToToolResponse sees merged user messages", func(t *testing.T) {
		// transformUserToToolResponse should see the merged user message
		// and convert it to a tool response
		messages := []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("first")}},
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("second")}},
			{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("followup")}},
		}

		req := &llm.Request{Messages: messages}

		require.NoError(t, MergeAdjacentUserMessages(req))
		require.NoError(t, MergeToolResultBlocks(req))
		require.NoError(t, TransformUserToToolResponse(req))

		// Should have 3 messages: merged user, assistant with tool_calls, tool
		require.Len(t, req.Messages, 3)
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Equal(t, "first\n\nsecond", *req.Messages[0].Content.Content)
		assert.Equal(t, "assistant", req.Messages[1].Role)
		assert.Len(t, req.Messages[1].ToolCalls, 1)
		assert.Equal(t, "tool", req.Messages[2].Role)
	})
}

// TestPipelineNilRequest verifies that nil requests are handled correctly
// by all transforms in the pipeline.
func TestPipelineNilRequest(t *testing.T) {
	// Each transform should return an error for nil request
	err := MergeAdjacentUserMessages(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request is nil")

	err = MergeToolResultBlocks(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request is nil")

	err = TransformUserToToolResponse(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request is nil")
}

// TestPipelineWithMiddlewareWrappers verifies that the middleware wrappers
// correctly call the underlying transform functions.
func TestPipelineWithMiddlewareWrappers(t *testing.T) {
	ctx := t.Context()

	t.Run("NewMergeAdjacentUserMessagesMiddleware", func(t *testing.T) {
		middleware := NewMergeAdjacentUserMessagesMiddleware()
		assert.NotNil(t, middleware)
		assert.Equal(t, "merge-adjacent-user-messages", middleware.Name())

		req := &llm.Request{
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("first")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("second")}},
			},
		}

		result, err := middleware.OnInboundLlmRequest(ctx, req)
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
		assert.Equal(t, "first\n\nsecond", *result.Messages[0].Content.Content)
	})

	t.Run("NewMergeToolResultBlocksMiddleware", func(t *testing.T) {
		middleware := NewMergeToolResultBlocksMiddleware()
		assert.NotNil(t, middleware)
		assert.Equal(t, "merge-tool-result-blocks", middleware.Name())

		req := &llm.Request{
			Messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("result")}, ToolCallID: lo.ToPtr("call_1")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("followup")}},
			},
		}

		result, err := middleware.OnInboundLlmRequest(ctx, req)
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
		assert.Contains(t, *result.Messages[0].Content.Content, "result")
		assert.Contains(t, *result.Messages[0].Content.Content, "followup")
	})

	t.Run("NewTransformUserToToolResponseMiddleware", func(t *testing.T) {
		middleware := NewTransformUserToToolResponseMiddleware()
		assert.NotNil(t, middleware)
		assert.Equal(t, "transform-user-to-tool-response", middleware.Name())

		req := &llm.Request{
			Messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("followup")}},
			},
		}

		result, err := middleware.OnInboundLlmRequest(ctx, req)
		require.NoError(t, err)
		assert.Len(t, result.Messages, 2)
		assert.Len(t, result.Messages[0].ToolCalls, 1)
		assert.Equal(t, "tool", result.Messages[1].Role)
	})
}

// TestPipelineComplexScenarios tests complex real-world scenarios.
func TestPipelineComplexScenarios(t *testing.T) {
	t.Run("multi-turn conversation with tool calls", func(t *testing.T) {
		// Simulates a conversation where:
		// 1. User asks a question
		// 2. Assistant calls a tool
		// 3. Tool returns result
		// 4. User provides follow-up
		// 5. Assistant responds
		// 6. User asks another question
		messages := []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("What's the weather?")}},
			{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Let me check.")}, ToolCalls: []llm.ToolCall{
				{ID: "call_weather", Type: "function", Function: llm.FunctionCall{Name: "get_weather", Arguments: "{}"}},
			}},
			{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("Sunny, 72°F")}, ToolCallID: lo.ToPtr("call_weather")},
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Thanks! What about tomorrow?")}},
			{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Tomorrow will be similar.")}},
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Great, thanks!")}},
		}

		req := &llm.Request{Messages: messages}

		require.NoError(t, MergeAdjacentUserMessages(req))
		require.NoError(t, MergeToolResultBlocks(req))
		require.NoError(t, TransformUserToToolResponse(req))

		// Expected: 5 messages
		// 1. user "What's the weather?"
		// 2. assistant with tool_calls
		// 3. tool merged with "Thanks! What about tomorrow?"
		// 4. assistant with tool_calls (for "Great, thanks!")
		// 5. tool "Great, thanks!"
		require.Len(t, req.Messages, 5)

		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Equal(t, "assistant", req.Messages[1].Role)
		assert.Equal(t, "tool", req.Messages[2].Role)
		assert.Contains(t, *req.Messages[2].Content.Content, "Sunny, 72°F")
		assert.Contains(t, *req.Messages[2].Content.Content, "Thanks! What about tomorrow?")
		assert.Equal(t, "assistant", req.Messages[3].Role)
		assert.Len(t, req.Messages[3].ToolCalls, 1)
		assert.Equal(t, "tool", req.Messages[4].Role)
	})

	t.Run("user provides multiple inputs before assistant responds", func(t *testing.T) {
		// Simulates a user sending multiple messages rapidly
		messages := []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("I have a question")}},
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("About the weather")}},
			{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Go ahead")}},
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Is it raining?")}},
		}

		req := &llm.Request{Messages: messages}

		require.NoError(t, MergeAdjacentUserMessages(req))
		require.NoError(t, MergeToolResultBlocks(req))
		require.NoError(t, TransformUserToToolResponse(req))

		// Expected: 3 messages
		// 1. merged user "Hello\n\nI have a question\n\nAbout the weather"
		// 2. assistant with tool_calls
		// 3. tool "Is it raining?"
		require.Len(t, req.Messages, 3)
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.True(t, strings.Contains(*req.Messages[0].Content.Content, "Hello"))
		assert.True(t, strings.Contains(*req.Messages[0].Content.Content, "I have a question"))
		assert.True(t, strings.Contains(*req.Messages[0].Content.Content, "About the weather"))
		assert.Equal(t, "assistant", req.Messages[1].Role)
		assert.Len(t, req.Messages[1].ToolCalls, 1)
		assert.Equal(t, "tool", req.Messages[2].Role)
	})
}
