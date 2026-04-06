package transforms

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestMergeAdjacentUserMessages(t *testing.T) {
	tests := []struct {
		name        string
		messages    []llm.Message
		wantLen     int
		wantContent func(t *testing.T, msgs []llm.Message)
		wantErr     bool
		errContain  string
	}{
		// Scenario 1.1: Empty messages → no-op
		{
			name:     "1.1 empty messages nil",
			messages: nil,
			wantLen:  0,
		},
		{
			name:     "1.1 empty messages slice",
			messages: []llm.Message{},
			wantLen:  0,
		},

		// Scenario 1.2: Single user message → no-op
		{
			name: "1.2 single user message",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "hello", *msgs[0].Content.Content)
			},
		},

		// Scenario 1.3: Two consecutive users, both string → merge with "\n\n"
		{
			name: "1.3 two consecutive users string",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("first")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("second")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "first\n\nsecond", *msgs[0].Content.Content)
			},
		},

		// Scenario 1.4: Three consecutive users, all string → all merged
		{
			name: "1.4 three consecutive users string",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("first")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("second")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("third")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "first\n\nsecond\n\nthird", *msgs[0].Content.Content)
			},
		},

		// Scenario 1.5: Users separated by assistant → NOT merged
		{
			name: "1.5 users separated by assistant",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user1")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user2")}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user1", *msgs[0].Content.Content)
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Equal(t, "user2", *msgs[2].Content.Content)
			},
		},

		// Scenario 1.6: Users separated by system → NOT merged
		{
			name: "1.6 users separated by system",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user1")}},
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("system prompt")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user2")}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user1", *msgs[0].Content.Content)
				assert.Equal(t, "system", msgs[1].Role)
				assert.Equal(t, "user2", *msgs[2].Content.Content)
			},
		},

		// Scenario 1.7: String + array content → converted to array with both as text parts
		{
			name: "1.7 string then array content",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("string content")}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("array content")},
					},
				}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Len(t, msgs[0].Content.MultipleContent, 2)
				assert.Equal(t, "text", msgs[0].Content.MultipleContent[0].Type)
				assert.Equal(t, "string content", *msgs[0].Content.MultipleContent[0].Text)
				assert.Equal(t, "text", msgs[0].Content.MultipleContent[1].Type)
				assert.Equal(t, "array content", *msgs[0].Content.MultipleContent[1].Text)
			},
		},

		// Scenario 1.8: Array + string content → converted to array
		{
			name: "1.8 array then string content",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("array content")},
					},
				}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("string content")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Len(t, msgs[0].Content.MultipleContent, 2)
				assert.Equal(t, "text", msgs[0].Content.MultipleContent[0].Type)
				assert.Equal(t, "array content", *msgs[0].Content.MultipleContent[0].Text)
				assert.Equal(t, "text", msgs[0].Content.MultipleContent[1].Type)
				assert.Equal(t, "string content", *msgs[0].Content.MultipleContent[1].Text)
			},
		},

		// Scenario 1.9: Array + array content → merged arrays
		{
			name: "1.9 array then array content",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("first array")},
					},
				}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("second array")},
					},
				}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Len(t, msgs[0].Content.MultipleContent, 2)
				assert.Equal(t, "first array", *msgs[0].Content.MultipleContent[0].Text)
				assert.Equal(t, "second array", *msgs[0].Content.MultipleContent[1].Text)
			},
		},

		// Scenario 1.10: Two groups of consecutive users
		{
			name: "1.10 two groups of consecutive users",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user1a")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user1b")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user2a")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user2b")}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user1a\n\nuser1b", *msgs[0].Content.Content)
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Equal(t, "user2a\n\nuser2b", *msgs[2].Content.Content)
			},
		},

		// Scenario 1.11: Left empty string → empty dropped, keep "hello"
		{
			name: "1.11 left empty string",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "hello", *msgs[0].Content.Content)
			},
		},

		// Scenario 1.12: Right empty string → empty dropped, keep "hello"
		{
			name: "1.12 right empty string",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "hello", *msgs[0].Content.Content)
			},
		},

		// Scenario 1.13: No messages → no-op (same as 1.1)
		{
			name:     "1.13 no messages",
			messages: nil,
			wantLen:  0,
		},

		// Scenario 1.14: Single user message → no-op (same as 1.2)
		{
			name: "1.14 single user message with array content",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("hello")},
					},
				}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Len(t, msgs[0].Content.MultipleContent, 1)
				assert.Equal(t, "hello", *msgs[0].Content.MultipleContent[0].Text)
			},
		},

		// Scenario 1.15: Two consecutive user messages with array content → merged content arrays
		{
			name: "1.15 two consecutive users with array content",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("first text")},
						{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/image1.png"}},
					},
				}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("second text")},
					},
				}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Len(t, msgs[0].Content.MultipleContent, 3)
				assert.Equal(t, "first text", *msgs[0].Content.MultipleContent[0].Text)
				assert.Equal(t, "image_url", msgs[0].Content.MultipleContent[1].Type)
				assert.Equal(t, "second text", *msgs[0].Content.MultipleContent[2].Text)
			},
		},

		// Scenario 1.16: Three consecutive user messages with array content → all merged
		{
			name: "1.16 three consecutive users with array content",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("first")},
					},
				}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("second")},
					},
				}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("third")},
					},
				}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Len(t, msgs[0].Content.MultipleContent, 3)
				assert.Equal(t, "first", *msgs[0].Content.MultipleContent[0].Text)
				assert.Equal(t, "second", *msgs[0].Content.MultipleContent[1].Text)
				assert.Equal(t, "third", *msgs[0].Content.MultipleContent[2].Text)
			},
		},

		// Scenario 1.17: Separated by assistant with tool_calls → NOT merged
		{
			name: "1.17 separated by assistant with tool_calls",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user1")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}, ToolCalls: []llm.ToolCall{{ID: "call_123", Type: "function", Function: llm.FunctionCall{Name: "get_weather"}}}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user2")}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user1", *msgs[0].Content.Content)
				assert.Equal(t, "assistant", msgs[1].Role)
				assert.Len(t, msgs[1].ToolCalls, 1)
				assert.Equal(t, "user2", *msgs[2].Content.Content)
			},
		},

		// Scenario 1.18: Separated by tool message → NOT merged
		{
			name: "1.18 separated by tool message",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user1")}},
				{Role: "tool", Content: llm.MessageContent{Content: lo.ToPtr("tool response")}, ToolCallID: lo.ToPtr("call_123")},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user2")}},
			},
			wantLen: 3,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "user1", *msgs[0].Content.Content)
				assert.Equal(t, "tool", msgs[1].Role)
				assert.Equal(t, "user2", *msgs[2].Content.Content)
			},
		},

		// Scenario 1.19: Separated by assistant → NOT merged (same as 1.5)
		{
			name: "1.19 separated by assistant again",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user1")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user2")}},
			},
			wantLen: 3,
		},

		// Scenario 1.20: String + array content merge (same as 1.7)
		{
			name: "1.20 string then array content again",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("text")}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("array")},
					},
				}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Len(t, msgs[0].Content.MultipleContent, 2)
			},
		},

		// Scenario 1.21: String + string merge (same as 1.3)
		{
			name: "1.21 two consecutive users string again",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("a")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("b")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "a\n\nb", *msgs[0].Content.Content)
			},
		},

		// Scenario 1.22: Array + array merge (same as 1.9)
		{
			name: "1.22 array then array content again",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("a")},
					},
				}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("b")},
					},
				}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Len(t, msgs[0].Content.MultipleContent, 2)
			},
		},

		// Scenario 1.23: Mixed items: two merge groups with assistant boundary
		{
			name: "1.23 mixed items two merge groups",
			messages: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("system prompt")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user1a")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user1b")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response1")}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("user2a")},
					},
				}},
				{Role: "user", Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("user2b")},
					},
				}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("response2")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user3")}},
			},
			wantLen: 6,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				assert.Equal(t, "system", msgs[0].Role)
				assert.Equal(t, "user", msgs[1].Role)
				assert.Equal(t, "user1a\n\nuser1b", *msgs[1].Content.Content)
				assert.Equal(t, "assistant", msgs[2].Role)
				assert.Equal(t, "user", msgs[3].Role)
				assert.Len(t, msgs[3].Content.MultipleContent, 2)
				assert.Equal(t, "assistant", msgs[4].Role)
				assert.Equal(t, "user", msgs[5].Role)
				assert.Equal(t, "user3", *msgs[5].Content.Content)
			},
		},

		// Additional edge cases
		{
			name: "both empty strings",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("")}},
			},
			wantLen: 1,
			wantContent: func(t *testing.T, msgs []llm.Message) {
				// Both empty, keeps first (which is empty)
				assert.True(t, msgs[0].Content.Content == nil || *msgs[0].Content.Content == "")
			},
		},
		{
			name:     "nil request returns error",
			messages: nil,
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &llm.Request{Messages: tt.messages}
			err := MergeAdjacentUserMessages(req)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
				return
			}

			require.NoError(t, err)
			assert.Len(t, req.Messages, tt.wantLen)
			if tt.wantContent != nil {
				tt.wantContent(t, req.Messages)
			}
		})
	}
}

func TestMergeAdjacentUserMessages_NilRequest(t *testing.T) {
	err := MergeAdjacentUserMessages(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request is nil")
}

func TestIsEmptyContent(t *testing.T) {
	tests := []struct {
		name     string
		content  llm.MessageContent
		expected bool
	}{
		{
			name:     "nil content string",
			content:  llm.MessageContent{Content: nil},
			expected: true,
		},
		{
			name:     "empty content string",
			content:  llm.MessageContent{Content: lo.ToPtr("")},
			expected: true,
		},
		{
			name:     "non-empty content string",
			content:  llm.MessageContent{Content: lo.ToPtr("hello")},
			expected: false,
		},
		{
			name:     "empty multiple content",
			content:  llm.MessageContent{MultipleContent: []llm.MessageContentPart{}},
			expected: true,
		},
		{
			name: "non-empty multiple content",
			content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("hello")},
				},
			},
			expected: false,
		},
		{
			name:     "empty struct",
			content:  llm.MessageContent{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isEmptyContent(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeUserMessages(t *testing.T) {
	tests := []struct {
		name     string
		first    llm.Message
		second   llm.Message
		validate func(t *testing.T, result llm.Message)
	}{
		{
			name:   "string + string",
			first:  llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("first")}},
			second: llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("second")}},
			validate: func(t *testing.T, result llm.Message) {
				assert.Equal(t, "first\n\nsecond", *result.Content.Content)
			},
		},
		{
			name:  "string + array",
			first: llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("string")}},
			second: llm.Message{Role: "user", Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("array")},
				},
			}},
			validate: func(t *testing.T, result llm.Message) {
				assert.Len(t, result.Content.MultipleContent, 2)
				assert.Equal(t, "string", *result.Content.MultipleContent[0].Text)
				assert.Equal(t, "array", *result.Content.MultipleContent[1].Text)
			},
		},
		{
			name: "array + string",
			first: llm.Message{Role: "user", Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("array")},
				},
			}},
			second: llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("string")}},
			validate: func(t *testing.T, result llm.Message) {
				assert.Len(t, result.Content.MultipleContent, 2)
				assert.Equal(t, "array", *result.Content.MultipleContent[0].Text)
				assert.Equal(t, "string", *result.Content.MultipleContent[1].Text)
			},
		},
		{
			name: "array + array",
			first: llm.Message{Role: "user", Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("first")},
				},
			}},
			second: llm.Message{Role: "user", Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("second")},
				},
			}},
			validate: func(t *testing.T, result llm.Message) {
				assert.Len(t, result.Content.MultipleContent, 2)
				assert.Equal(t, "first", *result.Content.MultipleContent[0].Text)
				assert.Equal(t, "second", *result.Content.MultipleContent[1].Text)
			},
		},
		{
			name:   "empty + non-empty",
			first:  llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("")}},
			second: llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
			validate: func(t *testing.T, result llm.Message) {
				assert.Equal(t, "hello", *result.Content.Content)
			},
		},
		{
			name:   "non-empty + empty",
			first:  llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
			second: llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("")}},
			validate: func(t *testing.T, result llm.Message) {
				assert.Equal(t, "hello", *result.Content.Content)
			},
		},
		{
			name:   "empty + empty",
			first:  llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("")}},
			second: llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("")}},
			validate: func(t *testing.T, result llm.Message) {
				// Both empty, keeps first
				assert.True(t, result.Content.Content == nil || *result.Content.Content == "")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeUserMessages(tt.first, tt.second)
			tt.validate(t, result)
		})
	}
}

func TestNewMergeAdjacentUserMessagesMiddleware(t *testing.T) {
	middleware := NewMergeAdjacentUserMessagesMiddleware()
	assert.NotNil(t, middleware)
	assert.Equal(t, "merge-adjacent-user-messages", middleware.Name())
}
