// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"strings"
	"testing"

	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/typeschema"
	"github.com/terramate-io/terramate/yaml"
)

// Only inputs with a prompt are part of a change. All other inputs of a bundle
// must be carried over from the existing config.
var promptedDefs = []*config.InputDefinition{
	promptedInput("name", "Name of the instance", "Name"),
	promptedInput("description", "Short description", "Description"),
}

func TestGenerateBundleYAMLKeepsInputsWithoutPrompt(t *testing.T) {
	existing := decodeBundleInstance(t, `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: example
  uuid: 6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55
spec:
  source: /bundles/example/v1
  inputs:
    # tmdoc: Name of the instance
    name: example
    # tmdoc: Gitignore template applied on creation
    gitignore_template: Terraform
    # tmdoc: Archive the instance, making it read-only
    archived: true
`)

	change := reconfigChange(nil, map[string]cty.Value{
		"name":        cty.StringVal("example"),
		"description": cty.StringVal("Set during the reconfigure"),
	})

	// The inputs without a prompt keep their position and comments, while the
	// newly set description is appended.
	assertBundleYAML(t, change, existing, nil, `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: example
  uuid: 6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55
spec:
  source: /bundles/example/v1
  inputs:
    # tmdoc: Name of the instance
    name: example
    # tmdoc: Gitignore template applied on creation
    gitignore_template: Terraform
    # tmdoc: Archive the instance, making it read-only
    archived: true
    # tmdoc: Short description
    description: Set during the reconfigure
`)
}

func TestGenerateBundleYAMLDropsResetInput(t *testing.T) {
	existing := decodeBundleInstance(t, `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: example
  uuid: 6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55
spec:
  source: /bundles/example/v1
  inputs:
    # tmdoc: Name of the instance
    name: example
    # tmdoc: Short description
    description: Instance created before the reconfigure
    # tmdoc: Archive the instance, making it read-only
    archived: true
`)

	// The description was reset in the form, so the change has no value for it.
	change := reconfigChange(nil, map[string]cty.Value{
		"name": cty.StringVal("example"),
	})

	assertBundleYAML(t, change, existing, nil, `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: example
  uuid: 6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55
spec:
  source: /bundles/example/v1
  inputs:
    # tmdoc: Name of the instance
    name: example
    # tmdoc: Archive the instance, making it read-only
    archived: true
`)
}

func TestGenerateBundleYAMLKeepsInputsWithoutPromptOfEnv(t *testing.T) {
	dev := &config.Environment{ID: "dev", Name: "Development"}
	prod := &config.Environment{ID: "prod", Name: "Production"}

	existing := decodeBundleInstance(t, `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: example
  uuid: 6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55
environments:
  dev:
    source: /bundles/example/v1
    inputs:
      # tmdoc: Name of the instance
      name: example-dev
      # tmdoc: Gitignore template applied on creation
      gitignore_template: Terraform
  prod:
    source: /bundles/example/v1
    inputs:
      # tmdoc: Name of the instance
      name: example-prod
      # tmdoc: Archive the instance, making it read-only
      archived: true
`)

	// Reconfiguring prod must not touch dev, and must keep the inputs without a
	// prompt of prod itself.
	change := reconfigChange(prod, map[string]cty.Value{
		"name": cty.StringVal("example-production"),
	})

	// The empty spec is what the encoder emits for a config that only has
	// environments, independent of the merge.
	assertBundleYAML(t, change, existing, []*config.Environment{dev, prod}, `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: example
  uuid: 6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55
spec: {}
environments:
  dev:
    source: /bundles/example/v1
    inputs:
      # tmdoc: Name of the instance
      name: example-dev
      # tmdoc: Gitignore template applied on creation
      gitignore_template: Terraform
  prod:
    source: /bundles/example/v1
    inputs:
      # tmdoc: Name of the instance
      name: example-production
      # tmdoc: Archive the instance, making it read-only
      archived: true
`)
}

func TestGenerateBundleYAMLPromoteKeepsInputsWithoutPrompt(t *testing.T) {
	dev := &config.Environment{ID: "dev", Name: "Development"}
	prod := &config.Environment{ID: "prod", Name: "Production"}

	existing := decodeBundleInstance(t, `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: example
  uuid: 6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55
environments:
  dev:
    source: /bundles/example/v1
    inputs:
      # tmdoc: Name of the instance
      name: example-dev
      # tmdoc: Gitignore template applied on creation
      gitignore_template: Terraform
`)

	// Promoting dev to prod: the inputs without a prompt must come along, since
	// the target environment does not have them yet.
	change := reconfigChange(prod, map[string]cty.Value{
		"name": cty.StringVal("example-prod"),
	})
	change.Kind = ChangePromote
	change.FromEnv = dev

	assertBundleYAML(t, change, existing, []*config.Environment{dev, prod}, `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: example
  uuid: 6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55
spec: {}
environments:
  dev:
    source: /bundles/example/v1
    inputs:
      # tmdoc: Name of the instance
      name: example-dev
      # tmdoc: Gitignore template applied on creation
      gitignore_template: Terraform
  prod:
    source: /bundles/example/v1
    inputs:
      # tmdoc: Name of the instance
      name: example-prod
      # tmdoc: Gitignore template applied on creation
      gitignore_template: Terraform
`)
}

func promptedInput(name, description, prompt string) *config.InputDefinition {
	return &config.InputDefinition{
		Name:        name,
		Description: description,
		Type:        &typeschema.PrimitiveType{Name: "string"},
		Prompt:      config.PromptConfig{Text: prompt},
	}
}

func reconfigChange(env *config.Environment, userValues map[string]cty.Value) Change {
	return Change{
		Kind:       ChangeReconfig,
		Name:       "example",
		UUID:       "6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55",
		Source:     "/bundles/example/v1",
		Env:        env,
		InputDefs:  promptedDefs,
		UserValues: userValues,
	}
}

func assertBundleYAML(t *testing.T, c Change, existing *yaml.BundleInstance, envs []*config.Environment, want string) {
	t.Helper()

	got, err := c.generateBundleYAML(existing, envs)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("unexpected config\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func decodeBundleInstance(t *testing.T, content string) *yaml.BundleInstance {
	t.Helper()

	var bundle yaml.BundleInstance
	if err := yaml.Decode(strings.NewReader(content), &bundle); err != nil {
		t.Fatal(err)
	}
	return &bundle
}
