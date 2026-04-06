package transforms

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestMergeToolResultBlocks(t *testing.T) {
	tests := []struct {
		name        string
		messages    []llm.Message
		wantLen     int
		wantContent func(t *testing.T, msgs []llm.Message)
	}{
		// Format A - Messages Array (Scenarios 2.1-2.9)

		// Scenario 2.1: Empty messages → no-op
		{
			name:     "2.1 empty messages nil",
			messages: nil,
			wantLen:  0,
		},
		{
			name:     "2.1 empty messages slice",
			messages: []llm.Message{},
			wantLen:  0,
		},

		// Scenario 2.2: No user messages (only assistant) → no-op
		{
			name: "2.2 no user messages only assistant",
			messages: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("system prompt")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "system", msgs[0].Role)
				assert.Equal(t, "assistant", msgs[1].Role)
			},
		},

		// Scenario 2.3: User with no tool_result (only text) → no-op
		{
			name: "2.3 user with no tool_result",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user", msgs[0].Role)
				assert.Equal(t, "hello", *msgs[0].Content.Content)
			},
		},

		// Scenario 2.4: User with string content → no-op (no tool before it)
		{
			name: "2.4 user with string content no tool",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user message")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user", msgs[0].Role)
				assert.Equal(t, "user message", *msgs[0].Content.Content)
			},
		},

		// Scenario 2.5: tool message + user with text → merge
		{
			name: "2.5 tool message + user with text",
			messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool result")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user text")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[0].Role)
				assert.Equal(t, "tool result\n\nPlease execute skill now:user text", *msgs[0].Content.Content)
				assert.Equal(t, "call_123", *msgs[0].ToolCallID)
			},
		},

		// Scenario 2.6: tool message + user with multiple text blocks → all text merged into tool
		{
			name: "2.6 tool message + user with multiple text blocks",
			messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool result")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("first text")},
						{Type: "text", Text: lo.ToPtr(" second text")},
					},
				}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[0].Role)
				assert.Equal(t, "tool result\n\nPlease execute skill now:first text second text", *msgs[0].Content.Content)
			},
		},

		// Scenario 2.7: tool with array content + user text → flatten and merge
		{
			name: "2.7 tool with array content + user text",
			messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("tool part 1")},
						{Type: "text", Text: lo.ToPtr(" tool part 2")},
					},
				}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user text")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[0].Role)
				assert.Equal(t, "tool part 1 tool part 2\n\nPlease execute skill now:user text", *msgs[0].Content.Content)
			},
		},

		// Scenario 2.8: tool message only, no following user → no-op
		{
			name: "2.8 tool message only no following user",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user message")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}, ToolCalls: []llm.ToolCall{{ID: "call_123", Type: "function", Function: llm.FunctionCall{Name: "get_weather"}}}},
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool result")}, ToolCallID: lo.ToPtr("call_123")},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user", msgs[0].Role)
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Equal(t, "tool", msgs[2].Role)
				assert.Equal(t, "tool result", *msgs[2].Content.Content)
			},
		},

		// Scenario 2.9: Multiple messages, only one tool+user pair → only that pair merged
		{
			name: "2.9 multiple messages only one tool+user pair",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user1")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response1")}},
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool result")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user2")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response2")}},
			},
			wantLen: 4,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user", msgs[0].Role)
				assert.Equal(t, "user1", *msgs[0].Content.Content)
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Equal(t, "tool", msgs[2].Role)
				assert.Equal(t, "tool result\n\nPlease execute skill now:user2", *msgs[2].Content.Content)
				assert.Equal(t, "assistant", msgs[3].Role)
			},
		},

		// Format B - Responses API mapped to Messages (Scenarios 2.10-2.16)

		// Scenario 2.10: Empty messages → no-op (same as 2.1)
		{
			name:     "2.10 empty messages responses api",
			messages: nil,
			wantLen:  0,
		},

		// Scenario 2.11: No tool messages → no-op
		{
			name: "2.11 no tool messages",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("follow-up")}},
			},
			wantLen: 3,
		},

		// Scenario 2.12: tool + user with input_text → merged, user removed
		{
			name: "2.12 tool + user with input_text",
			messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool output")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user input")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[0].Role)
				assert.Equal(t, "tool output\n\nPlease execute skill now:user input", *msgs[0].Content.Content)
			},
		},

		// Scenario 2.13: tool + user with multiple input_text → all merged into tool
		{
			name: "2.13 tool + user with multiple input_text",
			messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool output")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("input part 1")},
						{Type: "text", Text: lo.ToPtr(" input part 2")},
						{Type: "text", Text: lo.ToPtr(" input part 3")},
					},
				}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[0].Role)
				assert.Equal(t, "tool output\n\nPlease execute skill now:input part 1 input part 2 input part 3", *msgs[0].Content.Content)
			},
		},

		// Scenario 2.14: tool + assistant (not user) → no merge
		{
			name: "2.14 tool + assistant not user",
			messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool output")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("assistant response")}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[0].Role)
				assert.Equal(t, "tool output", *msgs[0].Content.Content)
				assert.Equal(t, "assistant", msgs[1].Role)
			},
		},

		// Scenario 2.15: tool + user with string content → merge (string content is valid)
		{
			name: "2.15 tool + user with string content",
			messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool output")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("string user content")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[0].Role)
				assert.Equal(t, "tool output\n\nPlease execute skill now:string user content", *msgs[0].Content.Content)
			},
		},

		// Scenario 2.16: Multiple tool+user pairs → each merged independently
		{
			name: "2.16 multiple tool+user pairs",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user1")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response1")}, ToolCalls: []llm.ToolCall{{ID: "call_1", Type: "function", Function: llm.FunctionCall{Name: "tool1"}}}},
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool1 result")}, ToolCallID: lo.ToPtr("call_1")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user2")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response2")}, ToolCalls: []llm.ToolCall{{ID: "call_2", Type: "function", Function: llm.FunctionCall{Name: "tool2"}}}},
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool2 result")}, ToolCallID: lo.ToPtr("call_2")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user3")}},
			},
			wantLen: 5,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user", msgs[0].Role)
				assert.Equal(t, "user1", *msgs[0].Content.Content)
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Equal(t, "tool", msgs[2].Role)
				assert.Equal(t, "tool1 result\n\nPlease execute skill now:user2", *msgs[2].Content.Content)
				assert.Equal(t, "assistant", msgs[3].Role)
				assert.Equal(t, "tool", msgs[4].Role)
				assert.Equal(t, "tool2 result\n\nPlease execute skill now:user3", *msgs[4].Content.Content)
			},
		},

		// Additional edge cases

		// Tool with empty content + user with text
		{
			name: "tool empty content + user text",
			messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user text")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[0].Role)
				assert.Equal(t, "\n\nPlease execute skill now:user text", *msgs[0].Content.Content)
			},
		},

		// Tool with text + user with empty content
		{
			name: "tool text + user empty content",
			messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool result")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[0].Role)
				assert.Equal(t, "tool result", *msgs[0].Content.Content)
			},
		},

		// Tool followed by system message (not user) → no merge
		{
			name: "tool followed by system message",
			messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool result")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("system message")}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[0].Role)
				assert.Equal(t, "tool result", *msgs[0].Content.Content)
				assert.Equal(t, "system", msgs[1].Role)
			},
		},

		// Two consecutive tool messages (no user between) → no merge
		{
			name: "two consecutive tool messages",
			messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool1 result")}, ToolCallID: lo.ToPtr("call_1")},
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool2 result")}, ToolCallID: lo.ToPtr("call_2")},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[0].Role)
				assert.Equal(t, "tool1 result", *msgs[0].Content.Content)
				assert.Equal(t, "tool", msgs[1].Role)
				assert.Equal(t, "tool2 result", *msgs[1].Content.Content)
			},
		},

		// Tool + user + user (only first user merged)
		{
			name: "tool + user + user only first merged",
			messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool result")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user1")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user2")}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[0].Role)
				assert.Equal(t, "tool result\n\nPlease execute skill now:user1", *msgs[0].Content.Content)
				assert.Equal(t, "user", msgs[1].Role)
				assert.Equal(t, "user2", *msgs[1].Content.Content)
			},
		},

		// User with non-text content parts (image_url) → only text parts merged
		{
			name: "user with image_url content part",
			messages: []llm.Message{
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool result")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("look at this")},
						{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/image.png"}},
					},
				}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[0].Role)
				assert.Equal(t, "tool result\n\nPlease execute skill now:look at this", *msgs[0].Content.Content)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &llm.Request{Messages: tt.messages}
			err := MergeToolResultBlocks(req)

			require.NoError(t, err)
			assert.Len(t, req.Messages, tt.wantLen)
			if tt.wantContent != nil {
				tt.wantContent(t, req.Messages)
			}
		})
	}
}

func TestMergeToolResultBlocks_NilRequest(t *testing.T) {
	err := MergeToolResultBlocks(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request is nil")
}

func TestExtractTextContent(t *testing.T) {
	tests := []struct {
		name     string
		msg      llm.Message
		expected string
	}{
		{
			name:     "string content",
			msg:      llm.Message{Content: llm.MessageContent{Content: lo.ToPtr("hello world")}},
			expected: "hello world",
		},
		{
			name: "array content single text",
			msg: llm.Message{Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("single text")},
				},
			}},
			expected: "single text",
		},
		{
			name: "array content multiple text parts",
			msg: llm.Message{Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("first")},
					{Type: "text", Text: lo.ToPtr(" second")},
				},
			}},
			expected: "first second",
		},
		{
			name: "array content mixed types",
			msg: llm.Message{Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("text part")},
					{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/image.png"}},
					{Type: "text", Text: lo.ToPtr(" more text")},
				},
			}},
			expected: "text part more text",
		},
		{
			name:     "empty string content",
			msg:      llm.Message{Content: llm.MessageContent{Content: lo.ToPtr("")}},
			expected: "",
		},
		{
			name: "empty array content",
			msg: llm.Message{Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{},
			}},
			expected: "",
		},
		{
			name: "array content with nil text",
			msg: llm.Message{Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: nil},
					{Type: "text", Text: lo.ToPtr("valid text")},
				},
			}},
			expected: "valid text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTextContent(tt.msg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeToolWithUser(t *testing.T) {
	tests := []struct {
		name     string
		tool     llm.Message
		user     llm.Message
		validate func(t *testing.T, result llm.Message)
	}{
		{
			name: "both string content",
			tool: llm.Message{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool text")}, ToolCallID: lo.ToPtr("call_123")},
			user: llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user text")}},
			validate: func(t *testing.T, result llm.Message) {
				assert.Equal(t, "tool", result.Role)
				assert.Equal(t, "tool text\n\nPlease execute skill now:user text", *result.Content.Content)
				assert.Equal(t, "call_123", *result.ToolCallID)
			},
		},
		{
			name: "tool array + user string",
			tool: llm.Message{Role: "tool", Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("tool part")},
				},
			}, ToolCallID: lo.ToPtr("call_123")},
			user: llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user text")}},
			validate: func(t *testing.T, result llm.Message) {
				assert.Equal(t, "tool part\n\nPlease execute skill now:user text", *result.Content.Content)
			},
		},
		{
			name: "tool string + user array",
			tool: llm.Message{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool text")}, ToolCallID: lo.ToPtr("call_123")},
			user: llm.Message{Role: "user", Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("user part")},
				},
			}},
			validate: func(t *testing.T, result llm.Message) {
				assert.Equal(t, "tool text\n\nPlease execute skill now:user part", *result.Content.Content)
			},
		},
		{
			name: "user empty content",
			tool: llm.Message{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool text")}, ToolCallID: lo.ToPtr("call_123")},
			user: llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("")}},
			validate: func(t *testing.T, result llm.Message) {
				assert.Equal(t, "tool text", *result.Content.Content)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeToolWithUser(tt.tool, tt.user)
			tt.validate(t, result)
		})
	}
}

func TestNewMergeToolResultBlocksMiddleware(t *testing.T) {
	middleware := NewMergeToolResultBlocksMiddleware()
	assert.NotNil(t, middleware)
	assert.Equal(t, "merge-tool-result-blocks", middleware.Name())
}
