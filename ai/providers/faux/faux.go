// Package faux is an in-process fake provider for tests: no network, scripted
// responses, realistic chunked streaming of the full event protocol, abort
// handling, and a prompt-cache simulation keyed by sessionId common-prefix.
// It is the executable specification of the streaming event contract.
//
// Ports: packages/ai/src/providers/faux.ts
package faux

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

const (
	defaultAPI          = "faux"
	defaultProvider     = "faux"
	defaultModelID      = "faux-1"
	defaultModelName    = "Faux Model"
	defaultBaseURL      = "http://localhost:0"
	defaultMinTokenSize = 3
	defaultMaxTokenSize = 5
)

// ModelDefinition describes one faux model.
type ModelDefinition struct {
	ID            string
	Name          string
	Reasoning     bool
	Input         []ai.Modality
	Cost          *ai.ModelCost
	ContextWindow int
	MaxTokens     int
}

// Text builds a text block.
func Text(text string) ai.TextContent { return ai.TextContent{Text: text} }

// Thinking builds a thinking block.
func Thinking(thinking string) ai.ThinkingContent { return ai.ThinkingContent{Thinking: thinking} }

// ToolCallOptions configures ToolCall.
type ToolCallOptions struct{ ID string }

// ToolCall builds a tool-call block.
func ToolCall(name string, arguments map[string]any, options *ToolCallOptions) ai.ToolCall {
	id := ""
	if options != nil {
		id = options.ID
	}
	if id == "" {
		id = randomID("tool")
	}
	return ai.ToolCall{ID: id, Name: name, Arguments: arguments}
}

// AssistantMessageOptions configures AssistantMessage.
type AssistantMessageOptions struct {
	StopReason   ai.StopReason
	ErrorMessage string
	ResponseID   string
	Timestamp    int64
}

// AssistantMessage builds a canned assistant message from text or blocks.
func AssistantMessage(content []ai.AssistantContentPart, options *AssistantMessageOptions) *ai.AssistantMessage {
	msg := &ai.AssistantMessage{
		Content:    content,
		Api:        defaultAPI,
		Provider:   defaultProvider,
		Model:      defaultModelID,
		StopReason: ai.StopReasonStop,
		Timestamp:  time.Now().UnixMilli(),
	}
	if options != nil {
		if options.StopReason != "" {
			msg.StopReason = options.StopReason
		}
		msg.ErrorMessage = options.ErrorMessage
		msg.ResponseID = options.ResponseID
		if options.Timestamp != 0 {
			msg.Timestamp = options.Timestamp
		}
	}
	return msg
}

// TextMessage builds a canned assistant message from a plain string.
func TextMessage(text string, options *AssistantMessageOptions) *ai.AssistantMessage {
	return AssistantMessage([]ai.AssistantContentPart{Text(text)}, options)
}

// State tracks how many stream calls the provider served.
type State struct {
	mu        sync.Mutex
	callCount int
}

// CallCount returns the number of stream calls served.
func (s *State) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callCount
}

func (s *State) increment() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callCount++
}

// ResponseFactory produces a response for one stream call.
type ResponseFactory func(ctx context.Context, chat ai.Context, options *ai.StreamOptions, state *State, model *ai.Model) (*ai.AssistantMessage, error)

// ResponseStep is a queued response: a literal message or a factory.
type ResponseStep struct {
	Message *ai.AssistantMessage
	Factory ResponseFactory
}

// Step wraps a literal message as a ResponseStep.
func Step(message *ai.AssistantMessage) ResponseStep { return ResponseStep{Message: message} }

// StepFunc wraps a factory as a ResponseStep.
func StepFunc(factory ResponseFactory) ResponseStep { return ResponseStep{Factory: factory} }

// Options configures the faux provider.
type Options struct {
	Api             string
	Provider        string
	Models          []ModelDefinition
	TokensPerSecond float64
	TokenSizeMin    int
	TokenSizeMax    int
	// Rand seeds chunking; nil uses a time-seeded source.
	Rand *rand.Rand
}

// Handle exposes the faux provider plus its scripting surface.
type Handle struct {
	Provider ai.Provider
	Api      string
	Models   []*ai.Model

	core *core
}

// GetModel returns the model with the given id, or the first model for "".
func (h *Handle) GetModel(modelID string) *ai.Model { return h.core.getModel(modelID) }

// State returns the shared call-count state.
func (h *Handle) State() *State { return h.core.state }

// SetResponses replaces the queued responses.
func (h *Handle) SetResponses(responses ...ResponseStep) { h.core.setResponses(responses) }

// AppendResponses appends queued responses.
func (h *Handle) AppendResponses(responses ...ResponseStep) { h.core.appendResponses(responses) }

// PendingResponseCount returns how many responses are still queued.
func (h *Handle) PendingResponseCount() int { return h.core.pendingResponseCount() }

type core struct {
	api             string
	provider        string
	minTokenSize    int
	maxTokenSize    int
	tokensPerSecond float64
	models          []*ai.Model
	state           *State

	mu          sync.Mutex
	pending     []ResponseStep
	promptCache map[string]string
	rng         *rand.Rand
}

// New creates a faux provider handle.
//
//	faux := faux.New(nil)
//	models := ai.CreateModels(nil)
//	models.SetProvider(faux.Provider)
//	faux.SetResponses(faux.Step(faux.TextMessage("hi", nil)))
func New(options *Options) *Handle {
	if options == nil {
		options = &Options{}
	}
	api := options.Api
	if api == "" {
		api = randomID(defaultAPI)
	}
	providerID := options.Provider
	if providerID == "" {
		providerID = defaultProvider
	}
	minToken := options.TokenSizeMin
	if minToken == 0 {
		minToken = defaultMinTokenSize
	}
	maxToken := options.TokenSizeMax
	if maxToken == 0 {
		maxToken = defaultMaxTokenSize
	}
	if minToken > maxToken {
		minToken = maxToken
	}
	if minToken < 1 {
		minToken = 1
	}
	rng := options.Rand
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	definitions := options.Models
	if len(definitions) == 0 {
		definitions = []ModelDefinition{{
			ID:            defaultModelID,
			Name:          defaultModelName,
			Input:         []ai.Modality{ai.ModalityText, ai.ModalityImage},
			ContextWindow: 128000,
			MaxTokens:     16384,
		}}
	}
	models := make([]*ai.Model, 0, len(definitions))
	for _, def := range definitions {
		name := def.Name
		if name == "" {
			name = def.ID
		}
		input := def.Input
		if input == nil {
			input = []ai.Modality{ai.ModalityText, ai.ModalityImage}
		}
		cost := ai.ModelCost{}
		if def.Cost != nil {
			cost = *def.Cost
		}
		contextWindow := def.ContextWindow
		if contextWindow == 0 {
			contextWindow = 128000
		}
		maxTokens := def.MaxTokens
		if maxTokens == 0 {
			maxTokens = 16384
		}
		models = append(models, &ai.Model{
			ID: def.ID, Name: name, Api: api, Provider: providerID,
			BaseURL: defaultBaseURL, Reasoning: def.Reasoning, Input: input,
			Cost: cost, ContextWindow: contextWindow, MaxTokens: maxTokens,
		})
	}

	c := &core{
		api:             api,
		provider:        providerID,
		minTokenSize:    minToken,
		maxTokenSize:    maxToken,
		tokensPerSecond: options.TokensPerSecond,
		models:          models,
		state:           &State{},
		promptCache:     map[string]string{},
		rng:             rng,
	}

	provider := ai.CreateProvider(ai.CreateProviderOptions{
		ID: providerID,
		Auth: ai.ProviderAuth{APIKey: &ai.APIKeyAuth{
			Name: "Faux",
			Resolve: func(context.Context, ai.APIKeyResolveInput) (*ai.AuthResult, error) {
				return &ai.AuthResult{}, nil
			},
		}},
		Models: models,
		Api:    ai.StreamFuncs{StreamFunc: c.stream, StreamSimpleFunc: c.streamSimple},
	})

	return &Handle{Provider: provider, Api: api, Models: models, core: c}
}

func (c *core) getModel(modelID string) *ai.Model {
	if modelID == "" {
		return c.models[0]
	}
	for _, m := range c.models {
		if m.ID == modelID {
			return m
		}
	}
	return nil
}

func (c *core) setResponses(responses []ResponseStep) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending = append([]ResponseStep(nil), responses...)
}

func (c *core) appendResponses(responses []ResponseStep) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending = append(c.pending, responses...)
}

func (c *core) pendingResponseCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending)
}

func (c *core) shiftResponse() (ResponseStep, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.pending) == 0 {
		return ResponseStep{}, false
	}
	step := c.pending[0]
	c.pending = c.pending[1:]
	return step, true
}

func (c *core) stream(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *ai.Stream {
	outer := ai.NewStream()
	step, hasStep := c.shiftResponse()
	c.state.increment()

	go func() {
		fail := func(err error) {
			message := c.errorMessage(err, model.ID)
			outer.Push(ai.ErrorEvent{Reason: ai.StopReasonError, Error: message})
		}

		if opts != nil && opts.OnResponse != nil {
			if err := opts.OnResponse(ctx, ai.ProviderResponse{Status: 200, Headers: map[string]string{}}, model); err != nil {
				fail(err)
				return
			}
		}
		if !hasStep {
			message := c.errorMessage(fmt.Errorf("No more faux responses queued"), model.ID)
			message = c.withUsageEstimate(message, chat, opts)
			outer.Push(ai.ErrorEvent{Reason: ai.StopReasonError, Error: message})
			return
		}

		resolved := step.Message
		if step.Factory != nil {
			var err error
			resolved, err = step.Factory(ctx, chat, opts, c.state, model)
			if err != nil {
				fail(err)
				return
			}
		}
		message := c.cloneMessage(resolved, model.ID)
		message = c.withUsageEstimate(message, chat, opts)
		c.streamWithDeltas(ctx, outer, message)
	}()

	return outer
}

func (c *core) streamSimple(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.SimpleStreamOptions) *ai.Stream {
	var base *ai.StreamOptions
	if opts != nil {
		base = &opts.StreamOptions
	}
	return c.stream(ctx, model, chat, base)
}

func (c *core) errorMessage(err error, modelID string) *ai.AssistantMessage {
	return &ai.AssistantMessage{
		Content:      []ai.AssistantContentPart{},
		Api:          c.api,
		Provider:     c.provider,
		Model:        modelID,
		StopReason:   ai.StopReasonError,
		ErrorMessage: err.Error(),
		Timestamp:    time.Now().UnixMilli(),
	}
}

func (c *core) cloneMessage(message *ai.AssistantMessage, modelID string) *ai.AssistantMessage {
	cloned := message.Clone()
	cloned.Api = c.api
	cloned.Provider = c.provider
	cloned.Model = modelID
	if cloned.Timestamp == 0 {
		cloned.Timestamp = time.Now().UnixMilli()
	}
	return cloned
}

func abortedMessage(partial *ai.AssistantMessage) *ai.AssistantMessage {
	aborted := partial.Clone()
	aborted.StopReason = ai.StopReasonAborted
	aborted.ErrorMessage = "Request was aborted"
	aborted.Timestamp = time.Now().UnixMilli()
	return aborted
}

func estimateTokens(text string) int { return (len([]rune(text)) + 3) / 4 }

func randomID(prefix string) string {
	return prefix + ":" + strconv.FormatInt(time.Now().UnixMilli(), 10) + ":" + strconv.FormatInt(rand.Int63(), 36)
}

func contentToText(blocks []ai.UserContentPart) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch b := block.(type) {
		case ai.TextContent:
			parts = append(parts, b.Text)
		case ai.ImageContent:
			parts = append(parts, fmt.Sprintf("[image:%s:%d]", b.MimeType, len(b.Data)))
		}
	}
	return strings.Join(parts, "\n")
}

func assistantContentToText(content []ai.AssistantContentPart) string {
	parts := make([]string, 0, len(content))
	for _, block := range content {
		switch b := block.(type) {
		case ai.TextContent:
			parts = append(parts, b.Text)
		case ai.ThinkingContent:
			parts = append(parts, b.Thinking)
		case ai.ToolCall:
			args, _ := json.Marshal(b.Arguments)
			parts = append(parts, b.Name+":"+string(args))
		}
	}
	return strings.Join(parts, "\n")
}

func messageToText(message ai.Message) string {
	switch m := message.(type) {
	case *ai.UserMessage:
		if m.Content.Plain != nil {
			return *m.Content.Plain
		}
		return contentToText(m.Content.Blocks)
	case *ai.AssistantMessage:
		return assistantContentToText(m.Content)
	case *ai.ToolResultMessage:
		parts := []string{m.ToolName}
		for _, block := range m.Content {
			parts = append(parts, contentToText([]ai.UserContentPart{block}))
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func serializeContext(chat ai.Context) string {
	var parts []string
	if chat.SystemPrompt != "" {
		parts = append(parts, "system:"+chat.SystemPrompt)
	}
	for _, message := range chat.Messages {
		parts = append(parts, string(message.MessageRole())+":"+messageToText(message))
	}
	if len(chat.Tools) > 0 {
		tools, _ := json.Marshal(chat.Tools)
		parts = append(parts, "tools:"+string(tools))
	}
	return strings.Join(parts, "\n\n")
}

func commonPrefixLength(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

// withUsageEstimate computes input/cacheRead/cacheWrite from the serialized
// prompt, simulating prompt caching by sessionId common prefix.
func (c *core) withUsageEstimate(message *ai.AssistantMessage, chat ai.Context, opts *ai.StreamOptions) *ai.AssistantMessage {
	promptText := serializeContext(chat)
	promptTokens := estimateTokens(promptText)
	outputTokens := estimateTokens(assistantContentToText(message.Content))
	input := promptTokens
	cacheRead := 0
	cacheWrite := 0

	sessionID := ""
	cacheRetention := ai.CacheRetentionShort
	if opts != nil {
		sessionID = opts.SessionID
		cacheRetention = opts.EffectiveCacheRetention()
	}

	if sessionID != "" && cacheRetention != ai.CacheRetentionNone {
		c.mu.Lock()
		previousPrompt, hadPrevious := c.promptCache[sessionID]
		c.promptCache[sessionID] = promptText
		c.mu.Unlock()
		if hadPrevious && previousPrompt != "" {
			cachedChars := commonPrefixLength(previousPrompt, promptText)
			cacheRead = estimateTokens(previousPrompt[:cachedChars])
			cacheWrite = estimateTokens(promptText[cachedChars:])
			input = promptTokens - cacheRead
			if input < 0 {
				input = 0
			}
		} else {
			cacheWrite = promptTokens
		}
	}

	out := message.Clone()
	out.Usage = ai.Usage{
		Input:       input,
		Output:      outputTokens,
		CacheRead:   cacheRead,
		CacheWrite:  cacheWrite,
		TotalTokens: input + outputTokens + cacheRead + cacheWrite,
	}
	return out
}

func (c *core) splitByTokenSize(text string) []string {
	if text == "" {
		return []string{""}
	}
	runes := []rune(text)
	var chunks []string
	index := 0
	for index < len(runes) {
		c.mu.Lock()
		tokenSize := c.minTokenSize + c.rng.Intn(c.maxTokenSize-c.minTokenSize+1)
		c.mu.Unlock()
		charSize := tokenSize * 4
		if charSize < 1 {
			charSize = 1
		}
		end := index + charSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[index:end]))
		index = end
	}
	return chunks
}

func (c *core) scheduleChunk(ctx context.Context, chunk string) error {
	if c.tokensPerSecond <= 0 {
		return ctx.Err()
	}
	delay := time.Duration(float64(estimateTokens(chunk)) / c.tokensPerSecond * float64(time.Second))
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// streamWithDeltas emits the full event protocol for a canned message:
// start, then per-block start/delta/end with realistic chunking, then the
// terminal done/error.
func (c *core) streamWithDeltas(ctx context.Context, stream *ai.Stream, message *ai.AssistantMessage) {
	partial := message.Clone()
	partial.Content = []ai.AssistantContentPart{}

	abort := func() {
		aborted := abortedMessage(partial)
		stream.Push(ai.ErrorEvent{Reason: ai.StopReasonAborted, Error: aborted})
	}

	if ctx.Err() != nil {
		abort()
		return
	}

	stream.Push(ai.StartEvent{Partial: partial.Clone()})

	for index, block := range message.Content {
		if ctx.Err() != nil {
			abort()
			return
		}

		switch b := block.(type) {
		case ai.ThinkingContent:
			partial.Content = append(partial.Content, ai.ThinkingContent{})
			stream.Push(ai.ThinkingStartEvent{ContentIndex: index, Partial: partial.Clone()})
			for _, chunk := range c.splitByTokenSize(b.Thinking) {
				if err := c.scheduleChunk(ctx, chunk); err != nil {
					abort()
					return
				}
				current := partial.Content[index].(ai.ThinkingContent)
				current.Thinking += chunk
				partial.Content[index] = current
				stream.Push(ai.ThinkingDeltaEvent{ContentIndex: index, Delta: chunk, Partial: partial.Clone()})
			}
			partial.Content[index] = b
			stream.Push(ai.ThinkingEndEvent{ContentIndex: index, Content: b.Thinking, Partial: partial.Clone()})

		case ai.TextContent:
			partial.Content = append(partial.Content, ai.TextContent{})
			stream.Push(ai.TextStartEvent{ContentIndex: index, Partial: partial.Clone()})
			for _, chunk := range c.splitByTokenSize(b.Text) {
				if err := c.scheduleChunk(ctx, chunk); err != nil {
					abort()
					return
				}
				current := partial.Content[index].(ai.TextContent)
				current.Text += chunk
				partial.Content[index] = current
				stream.Push(ai.TextDeltaEvent{ContentIndex: index, Delta: chunk, Partial: partial.Clone()})
			}
			partial.Content[index] = b
			stream.Push(ai.TextEndEvent{ContentIndex: index, Content: b.Text, Partial: partial.Clone()})

		case ai.ToolCall:
			partial.Content = append(partial.Content, ai.ToolCall{ID: b.ID, Name: b.Name, Arguments: map[string]any{}})
			stream.Push(ai.ToolCallStartEvent{ContentIndex: index, Partial: partial.Clone()})
			args, _ := json.Marshal(b.Arguments)
			for _, chunk := range c.splitByTokenSize(string(args)) {
				if err := c.scheduleChunk(ctx, chunk); err != nil {
					abort()
					return
				}
				stream.Push(ai.ToolCallDeltaEvent{ContentIndex: index, Delta: chunk, Partial: partial.Clone()})
			}
			partial.Content[index] = b
			stream.Push(ai.ToolCallEndEvent{ContentIndex: index, ToolCall: b, Partial: partial.Clone()})
		}
	}

	if message.StopReason == ai.StopReasonError || message.StopReason == ai.StopReasonAborted {
		stream.Push(ai.ErrorEvent{Reason: message.StopReason, Error: message})
		return
	}

	stream.Push(ai.DoneEvent{Reason: message.StopReason, Message: message})
}
