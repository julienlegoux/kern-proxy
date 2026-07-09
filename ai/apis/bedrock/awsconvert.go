package bedrock

// Ports: packages/ai/src/api/bedrock-converse-stream.ts

// Converts the logical wireRequest (the shape exposed to OnPayload) into the
// AWS SDK's typed *bedrockruntime.ConverseStreamInput for the actual client
// call. This indirection exists so OnPayload sees/replaces a plain,
// JSON-shaped value (matching upstream's commandInput object) rather than
// the AWS SDK's document.Interface-wrapped union types.

import (
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

func toConverseStreamInput(req *wireRequest) *bedrockruntime.ConverseStreamInput {
	input := &bedrockruntime.ConverseStreamInput{
		ModelId:  &req.ModelID,
		Messages: toAWSMessages(req.Messages),
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens:   toInt32Ptr(req.InferenceConfig.MaxTokens),
			Temperature: toFloat32Ptr(req.InferenceConfig.Temperature),
		},
	}
	if len(req.System) > 0 {
		input.System = toAWSSystemBlocks(req.System)
	}
	if req.ToolConfig != nil {
		input.ToolConfig = toAWSToolConfig(req.ToolConfig)
	}
	if len(req.AdditionalModelRequestFields) > 0 {
		input.AdditionalModelRequestFields = document.NewLazyDocument(req.AdditionalModelRequestFields)
	}
	if len(req.RequestMetadata) > 0 {
		input.RequestMetadata = req.RequestMetadata
	}
	return input
}

func toInt32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	i := int32(*v)
	return &i
}

func toFloat32Ptr(v *float64) *float32 {
	if v == nil {
		return nil
	}
	f := float32(*v)
	return &f
}

func toAWSMessages(messages []wireMessage) []types.Message {
	out := make([]types.Message, len(messages))
	for i, m := range messages {
		role := types.ConversationRoleUser
		if m.Role == "assistant" {
			role = types.ConversationRoleAssistant
		}
		out[i] = types.Message{Role: role, Content: toAWSContentBlocks(m.Content)}
	}
	return out
}

func toAWSContentBlocks(blocks []wireContentBlock) []types.ContentBlock {
	out := make([]types.ContentBlock, 0, len(blocks))
	for _, b := range blocks {
		switch {
		case b.Image != nil:
			out = append(out, &types.ContentBlockMemberImage{Value: types.ImageBlock{
				Format: types.ImageFormat(b.Image.Format),
				Source: &types.ImageSourceMemberBytes{Value: b.Image.Source.Bytes},
			}})
		case b.ToolUse != nil:
			out = append(out, &types.ContentBlockMemberToolUse{Value: types.ToolUseBlock{
				ToolUseId: &b.ToolUse.ToolUseID,
				Name:      &b.ToolUse.Name,
				Input:     document.NewLazyDocument(nonNilMap(b.ToolUse.Input)),
			}})
		case b.ToolResult != nil:
			out = append(out, &types.ContentBlockMemberToolResult{Value: types.ToolResultBlock{
				ToolUseId: &b.ToolResult.ToolUseID,
				Content:   toAWSToolResultContent(b.ToolResult.Content),
				Status:    toAWSToolResultStatus(b.ToolResult.Status),
			}})
		case b.ReasoningContent != nil:
			out = append(out, &types.ContentBlockMemberReasoningContent{Value: toAWSReasoningContent(b.ReasoningContent)})
		case b.CachePoint != nil:
			out = append(out, &types.ContentBlockMemberCachePoint{Value: toAWSCachePoint(b.CachePoint)})
		default:
			out = append(out, &types.ContentBlockMemberText{Value: b.Text})
		}
	}
	return out
}

func nonNilMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func toAWSToolResultStatus(status string) types.ToolResultStatus {
	if status == "error" {
		return types.ToolResultStatusError
	}
	return ""
}

func toAWSToolResultContent(blocks []wireToolResultContent) []types.ToolResultContentBlock {
	out := make([]types.ToolResultContentBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.Image != nil {
			out = append(out, &types.ToolResultContentBlockMemberImage{Value: types.ImageBlock{
				Format: types.ImageFormat(b.Image.Format),
				Source: &types.ImageSourceMemberBytes{Value: b.Image.Source.Bytes},
			}})
			continue
		}
		out = append(out, &types.ToolResultContentBlockMemberText{Value: b.Text})
	}
	return out
}

func toAWSReasoningContent(rc *wireReasoningContent) types.ReasoningContentBlock {
	if rc.ReasoningText == nil {
		return &types.ReasoningContentBlockMemberReasoningText{}
	}
	block := types.ReasoningTextBlock{Text: &rc.ReasoningText.Text}
	if rc.ReasoningText.Signature != "" {
		block.Signature = &rc.ReasoningText.Signature
	}
	return &types.ReasoningContentBlockMemberReasoningText{Value: block}
}

func toAWSCachePoint(cp *wireCachePoint) types.CachePointBlock {
	block := types.CachePointBlock{Type: types.CachePointType(cp.Type)}
	if cp.TTL != "" {
		block.Ttl = types.CacheTTL(cp.TTL)
	}
	return block
}

func toAWSSystemBlocks(blocks []wireSystemBlock) []types.SystemContentBlock {
	out := make([]types.SystemContentBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.CachePoint != nil {
			out = append(out, &types.SystemContentBlockMemberCachePoint{Value: toAWSCachePoint(b.CachePoint)})
			continue
		}
		out = append(out, &types.SystemContentBlockMemberText{Value: b.Text})
	}
	return out
}

func toAWSToolConfig(tc *wireToolConfig) *types.ToolConfiguration {
	tools := make([]types.Tool, len(tc.Tools))
	for i, t := range tc.Tools {
		spec := types.ToolSpecification{
			Name:        &t.ToolSpec.Name,
			InputSchema: &types.ToolInputSchemaMemberJson{Value: document.NewLazyDocument(nonNilMap(t.ToolSpec.InputSchema.JSON))},
		}
		if t.ToolSpec.Description != "" {
			spec.Description = &t.ToolSpec.Description
		}
		tools[i] = &types.ToolMemberToolSpec{Value: spec}
	}

	config := &types.ToolConfiguration{Tools: tools}
	if tc.ToolChoice != nil {
		switch {
		case tc.ToolChoice.Tool != nil:
			config.ToolChoice = &types.ToolChoiceMemberTool{Value: types.SpecificToolChoice{Name: &tc.ToolChoice.Tool.Name}}
		case tc.ToolChoice.Any != nil:
			config.ToolChoice = &types.ToolChoiceMemberAny{}
		case tc.ToolChoice.Auto != nil:
			config.ToolChoice = &types.ToolChoiceMemberAuto{}
		}
	}
	return config
}
