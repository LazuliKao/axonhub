package transforms

import (
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestTransformUserToToolResponse(t *testing.T) {
	tests := []struct {
		name        string
		messages    []llm.Message
		wantLen     int
		wantContent func(t *testing.T, msgs []llm.Message)
	}{
		// Scenario 3.1: Empty messages → no-op
		{
			name:     "3.1 empty messages nil",
			messages: nil,
			wantLen:  0,
		},
		{
			name:     "3.1 empty messages slice",
			messages: []llm.Message{},
			wantLen:  0,
		},

		// Scenario 3.2: No assistant (only user) → no-op
		{
			name: "3.2 no assistant only user",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user", msgs[0].Role)
				assert.Equal(t, "hello", *msgs[0].Content.Content)
			},
		},

		// Scenario 3.3: User after assistant → inject tool_calls, convert user to tool
		{
			name: "3.3 user after assistant",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("follow-up")}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user", msgs[0].Role)
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Len(t, msgs[1].ToolCalls, 1)
				assert.Equal(t, "function", msgs[1].ToolCalls[0].Type)
				assert.Equal(t, "helper_temp_xxx_callback", msgs[1].ToolCalls[0].Function.Name)
				assert.Contains(t, msgs[1].ToolCalls[0].ID, "call_")
				assert.Equal(t, "tool", msgs[2].Role)
				assert.Equal(t, msgs[1].ToolCalls[0].ID, *msgs[2].ToolCallID)
				assert.Equal(t, "follow-up", *msgs[2].Content.Content)
			},
		},

		// Scenario 3.4: Multiple users after assistant → multiple tool_calls injected
		{
			name: "3.4 multiple users after assistant",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("follow-up 1")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("follow-up 2")}},
			},
			wantLen: 4,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Len(t, msgs[1].ToolCalls, 2)
				assert.Equal(t, "tool", msgs[2].Role)
				assert.Equal(t, "tool", msgs[3].Role)
				assert.NotEqual(t, *msgs[2].ToolCallID, *msgs[3].ToolCallID)
			},
		},

		// Scenario 3.5: User after tool (backtrack) → additional tool_call injected
		{
			name: "3.5 user after tool backtrack",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}, ToolCalls: []llm.ToolCall{
					{ID: "call_original", Type: "function", Function: llm.FunctionCall{Name: "get_weather", Arguments: "{}"}},
				}},
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("weather data")}, ToolCallID: lo.ToPtr("call_original")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("thanks")}},
			},
			wantLen: 4,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Len(t, msgs[1].ToolCalls, 1)
				assert.Equal(t, "user", msgs[3].Role)
			},
		},

		// Scenario 3.6: Assistant already has tool_use content → no-op
		{
			name: "3.6 assistant already has tool_use content",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "assistant", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "tool_use", Text: lo.ToPtr("tool content")},
					},
				}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("follow-up")}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user", msgs[2].Role)
			},
		},

		// Scenario 3.7: User already has tool_result → skip
		{
			name: "3.7 user already has tool_result",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "tool_result", Text: lo.ToPtr("result")},
					},
				}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user", msgs[2].Role)
				assert.Equal(t, "tool_result", msgs[2].Content.MultipleContent[0].Type)
			},
		},

		// Scenario 3.8: User with array content (pure text) → flatten to string with "User Input：" prefix
		{
			name: "3.8 user with array content pure text",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("part 1")},
						{Type: "text", Text: lo.ToPtr("part 2")},
					},
				}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[2].Role)
				assert.NotNil(t, msgs[2].Content.Content)
				assert.Contains(t, *msgs[2].Content.Content, "User Input：part 1")
				assert.Contains(t, *msgs[2].Content.Content, "User Input：part 2")
			},
		},

		// Scenario 3.9: User with mixed content (text + image) → preserve array, just change role
		{
			name: "3.9 user with mixed content text and image",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("look at this")},
						{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/image.png"}},
					},
				}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[2].Role)
				assert.Len(t, msgs[2].Content.MultipleContent, 2)
				assert.Equal(t, "text", msgs[2].Content.MultipleContent[0].Type)
				assert.Equal(t, "image_url", msgs[2].Content.MultipleContent[1].Type)
			},
		},

		// Scenario 3.10: User with plain string content → keep as-is, change role to tool
		{
			name: "3.10 user with plain string content",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("plain text follow-up")}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[2].Role)
				assert.Equal(t, "plain text follow-up", *msgs[2].Content.Content)
			},
		},

		// Scenario 3.11: Empty messages → no-op (Responses API)
		{
			name:     "3.11 empty messages responses api",
			messages: nil,
			wantLen:  0,
		},

		// Scenario 3.12: No assistant → no-op (Responses API)
		{
			name: "3.12 no assistant responses api",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("system")}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user", msgs[0].Role)
				assert.Equal(t, "system", msgs[1].Role)
			},
		},

		// Scenario 3.13: User after assistant → inject tool_calls, convert user to tool (Responses API)
		{
			name: "3.13 user after assistant responses api",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("follow-up")}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Len(t, msgs[0].ToolCalls, 1)
				assert.Equal(t, "tool", msgs[1].Role)
			},
		},

		// Scenario 3.14: Multiple users after assistant → multiple tool_calls (Responses API)
		{
			name: "3.14 multiple users after assistant responses api",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("follow-up 1")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("follow-up 2")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("follow-up 3")}},
			},
			wantLen: 4,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Len(t, msgs[0].ToolCalls, 3)
				assert.Equal(t, "tool", msgs[1].Role)
				assert.Equal(t, "tool", msgs[2].Role)
				assert.Equal(t, "tool", msgs[3].Role)
			},
		},

		// Scenario 3.15: Already has tool_calls → no-op (Responses API)
		{
			name: "3.15 already has tool_calls responses api",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}, ToolCalls: []llm.ToolCall{
					{ID: "call_123", Type: "function", Function: llm.FunctionCall{Name: "get_weather"}},
				}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("follow-up")}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Len(t, msgs[0].ToolCalls, 1)
				assert.Equal(t, "user", msgs[1].Role)
			},
		},

		// Scenario 3.16: User after tool message → no-op (already in tool result context)
		{
			name: "3.16 user after tool message",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}, ToolCalls: []llm.ToolCall{
					{ID: "call_123", Type: "function", Function: llm.FunctionCall{Name: "get_weather"}},
				}},
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("weather data")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("thanks")}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user", msgs[2].Role)
			},
		},

		// Scenario 3.17: Assistant has tool_calls between it and user → no-op
		{
			name: "3.17 assistant has tool_calls between it and user",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}, ToolCalls: []llm.ToolCall{
					{ID: "call_123", Type: "function", Function: llm.FunctionCall{Name: "get_weather"}},
				}},
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("result")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("final response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("thanks")}},
			},
			wantLen: 4,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Len(t, msgs[0].ToolCalls, 1)
				assert.Len(t, msgs[2].ToolCalls, 1)
				assert.Equal(t, "tool", msgs[3].Role)
			},
		},

		// Scenario 3.18: User with multiple text parts → flatten with "User Input：" prefix
		{
			name: "3.18 user with multiple text parts",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("first")},
						{Type: "text", Text: lo.ToPtr("second")},
						{Type: "text", Text: lo.ToPtr("third")},
					},
				}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[1].Role)
				assert.NotNil(t, msgs[1].Content.Content)
				flattened := *msgs[1].Content.Content
				assert.True(t, strings.Contains(flattened, "User Input：first"))
				assert.True(t, strings.Contains(flattened, "User Input：second"))
				assert.True(t, strings.Contains(flattened, "User Input：third"))
			},
		},

		// Scenario 3.19: User with string content → keep as-is, change role
		{
			name: "3.19 user with string content",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("simple string")}},
			},
			wantLen: 2,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "tool", msgs[1].Role)
				assert.Equal(t, "simple string", *msgs[1].Content.Content)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &llm.Request{Messages: tt.messages}
			err := TransformUserToToolResponse(req)

			require.NoError(t, err)
			assert.Len(t, req.Messages, tt.wantLen)
			if tt.wantContent != nil {
				tt.wantContent(t, req.Messages)
			}
		})
	}
}

func TestTransformUserToToolResponse_NilRequest(t *testing.T) {
	err := TransformUserToToolResponse(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request is nil")
}

func TestNewTransformUserToToolResponseMiddleware(t *testing.T) {
	middleware := NewTransformUserToToolResponseMiddleware()
	assert.NotNil(t, middleware)
	assert.Equal(t, "transform-user-to-tool-response", middleware.Name())
}

func TestFindTransformableAssistant(t *testing.T) {
	tests := []struct {
		name     string
		messages []llm.Message
		wantIdx  int
	}{
		{
			name:     "empty messages",
			messages: []llm.Message{},
			wantIdx:  -1,
		},
		{
			name: "no assistant",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
			},
			wantIdx: -1,
		},
		{
			name: "assistant first",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
			},
			wantIdx: 0,
		},
		{
			name: "assistant in middle",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("bye")}},
			},
			wantIdx: 1,
		},
		{
			name: "assistant with tool_calls skipped",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("hi")}, ToolCalls: []llm.ToolCall{{ID: "call_1"}}},
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("result")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
			},
			wantIdx: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := findTransformableAssistant(tt.messages)
			if tt.wantIdx == -1 {
				assert.Nil(t, ref)
			} else {
				require.NotNil(t, ref)
				assert.Equal(t, tt.wantIdx, ref.index)
			}
		})
	}
}

func TestHasToolUseContent(t *testing.T) {
	tests := []struct {
		name     string
		content  llm.MessageContent
		expected bool
	}{
		{
			name:     "string content",
			content:  llm.MessageContent{Content: lo.ToPtr("hello")},
			expected: false,
		},
		{
			name: "array without tool_use",
			content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("hello")},
				},
			},
			expected: false,
		},
		{
			name: "array with tool_use",
			content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "tool_use", Text: lo.ToPtr("tool content")},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasToolUseContent(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasToolResultContent(t *testing.T) {
	tests := []struct {
		name     string
		content  llm.MessageContent
		expected bool
	}{
		{
			name:     "string content",
			content:  llm.MessageContent{Content: lo.ToPtr("hello")},
			expected: false,
		},
		{
			name: "array without tool_result",
			content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("hello")},
				},
			},
			expected: false,
		},
		{
			name: "array with tool_result",
			content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "tool_result", Text: lo.ToPtr("result")},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasToolResultContent(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateCallID(t *testing.T) {
	id1, err := generateCallID()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(id1, "call_"))
	assert.Len(t, id1, 21) // "call_" (5) + 16 hex chars (8 bytes)

	id2, err := generateCallID()
	require.NoError(t, err)
	assert.NotEqual(t, id1, id2)
}
