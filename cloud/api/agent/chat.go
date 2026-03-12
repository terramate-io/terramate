// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package agent provides types and helpers for the agent API.
package agent

const (
	// HeaderProvider sets the LLM provider.
	HeaderProvider = "X-Tmagent-Provider"
	// HeaderAPIKey sets the LLM API key.
	HeaderAPIKey = "X-Tmagent-Api-Key"
	// HeaderModel sets the LLM model.
	HeaderModel = "X-Tmagent-Model"
)

// ChatMessageDTO represents a single message in the conversation history.
// For assistant messages, ToolCalls carries any tool invocations the LLM made.
// For user messages, ToolResults carries the outcomes of those invocations.
type ChatMessageDTO struct {
	Role        string           `json:"role"` // "user" or "assistant"
	Content     string           `json:"content"`
	ToolCalls   []ToolCall       `json:"tool_calls,omitempty"`
	ToolResults []ToolCallResult `json:"tool_results,omitempty"`
}

// ChatResponse is the JSON response from the agent chat endpoint.
type ChatResponse struct {
	Text      string     `json:"text"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}
