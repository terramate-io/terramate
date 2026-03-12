// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package agent

import "encoding/json"

// ToolCall represents a tool invocation returned by the LLM.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolCallResult represents the outcome of executing a tool call on the client.
// ToolCallID matches the ToolCall.ID so the LLM can correlate results.
type ToolCallResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}
