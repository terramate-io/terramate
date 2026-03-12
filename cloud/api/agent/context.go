// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package agent

// BundleDefinitionDTO is the JSON representation of a bundle definition.
type BundleDefinitionDTO struct {
	Source      string     `json:"source"`
	Name        string     `json:"name"`
	Class       string     `json:"class"`
	Version     string     `json:"version"`
	Description string     `json:"description,omitempty"`
	Inputs      []InputDTO `json:"inputs,omitempty"`
}

// InputDTO is the JSON representation of a bundle input definition.
type InputDTO struct {
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Description string     `json:"description,omitempty"`
	Default     string     `json:"default,omitempty"`
	Prompt      *PromptDTO `json:"prompt,omitempty"`
}

// PromptDTO is the JSON representation of an input's prompt configuration.
type PromptDTO struct {
	Text        string `json:"text,omitempty"`
	Options     string `json:"options,omitempty"`
	Multiline   string `json:"multiline,omitempty"`
	Multiselect string `json:"multiselect,omitempty"`
}

// BundleInstanceDTO is the JSON representation of an existing bundle instance
// loaded from the project. The location uniquely identifies the instance.
type BundleInstanceDTO struct {
	Location         string            `json:"location"`
	DefinitionSource string            `json:"definition_source"`
	Name             string            `json:"name"`
	EnvID            string            `json:"env_id,omitempty"`
	Inputs           map[string]string `json:"inputs,omitempty"`
}
