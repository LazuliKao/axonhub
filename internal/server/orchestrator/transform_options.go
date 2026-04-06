package orchestrator

import (
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/transforms"
)

// applyTransformOptions applies channel transform options to create a new llm.Request.
// It creates a new request instead of modifying the original one.
func applyTransformOptions(req *llm.Request, channelSettings *objects.ChannelSettings) *llm.Request {
	if channelSettings == nil {
		return req
	}

	transformOptions := channelSettings.TransformOptions

	if !transformOptions.ForceArrayInstructions &&
		!transformOptions.ForceArrayInputs &&
		!transformOptions.ReplaceDeveloperRoleWithSystem {
		return req
	}

	newReq := *req

	if transformOptions.ForceArrayInstructions {
		newReq.TransformOptions.ArrayInstructions = lo.ToPtr(true)
	}

	if transformOptions.ForceArrayInputs {
		newReq.TransformOptions.ArrayInputs = lo.ToPtr(true)
	}

	if transformOptions.ReplaceDeveloperRoleWithSystem {
		newReq.Messages = replaceDeveloperRoleWithSystem(newReq.Messages)
	}

	return &newReq
}

// replaceDeveloperRoleWithSystem replaces developer role with system in messages.
func replaceDeveloperRoleWithSystem(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return messages
	}

	replaced := false

	result := make([]llm.Message, len(messages))
	for i, msg := range messages {
		if strings.EqualFold(msg.Role, "developer") {
			msg.Role = "system"
			replaced = true
		}

		result[i] = msg
	}

	if !replaced {
		return messages
	}

	return result
}

// applyTransformMiddlewares returns transform middlewares based on channel config flags.
// The middlewares are applied in order: mergeAdjacentUserMessages → mergeToolResultBlocks → transformUserToToolResponse.
// Default values when flags are nil: all transforms are disabled (only enable when explicitly set to true).
func applyTransformMiddlewares(channelSettings *objects.ChannelSettings) []pipeline.Middleware {
	if channelSettings == nil {
		return nil
	}

	opts := channelSettings.TransformOptions
	var middlewares []pipeline.Middleware

	// Merge adjacent user messages first (order matters)
	if opts.MergeAdjacentUserMessages != nil && *opts.MergeAdjacentUserMessages {
		middlewares = append(middlewares, transforms.NewMergeAdjacentUserMessagesMiddleware())
	}

	// Then merge tool result blocks
	if opts.MergeToolBlocks != nil && *opts.MergeToolBlocks {
		middlewares = append(middlewares, transforms.NewMergeToolResultBlocksMiddleware())
	}

	// Finally transform user messages to tool responses
	if opts.TransformUserMessages != nil && *opts.TransformUserMessages {
		middlewares = append(middlewares, transforms.NewTransformUserToToolResponseMiddleware())
	}

	return middlewares
}
