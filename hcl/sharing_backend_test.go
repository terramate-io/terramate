// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/test"
	errtest "github.com/terramate-io/terramate/test/errors"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestParserSharingBackend(t *testing.T) {
	t.Run("sharing_backend with no labels", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Doc(
				Block("sharing_backend",
					Expr("type", "terraform"),
					Expr("command", `["whatever"]`),
					Str("filename", "test.tf"),
				),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, "expected a single label but 0 given"))
	})

	t.Run("sharing_backend with more than 1 label", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("sharing_backend",
				Labels("label_1", "label_2"),
				Expr("type", "terraform"),
				Expr("command", `["whatever"]`),
				Str("filename", "test.tf"),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, "expected a single label but 2 given"))
	})

	t.Run("sharing_backend with no filename", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("sharing_backend",
				Labels("common-backend"),
				Expr("type", "terraform"),
				Expr("command", `["whatever"]`),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, `attribute "sharing_backend.filename" is required`))
	})

	t.Run("sharing_backend with no command", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("sharing_backend",
				Labels("common-backend"),
				Expr("type", "terraform"),
				Str("filename", "something.tf"),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, `attribute "sharing_backend.command" is required`))
	})

	t.Run("sharing_backend with empty command", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("sharing_backend",
				Labels("common-backend"),
				Expr("type", "terraform"),
				Str("filename", "something.tf"),
				Expr("command", `[]`),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, `"sharing_backend.command" must be a non-empty list of strings`))
	})

	t.Run("sharing_backend with invalid command type", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("sharing_backend",
				Labels("common-backend"),
				Expr("type", "terraform"),
				Str("filename", "something.tf"),
				Expr("command", `1`),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(`"sharing_backend.command" must be a list(string) but "number" given`))
	})

	t.Run("sharing_backend with no type", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("sharing_backend",
				Labels("common-backend"),
				Expr("command", `["whatever"]`),
				Str("filename", "something.tf"),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, `attribute "sharing_backend.type" is required`))
	})

	t.Run("sharing_backend with invalid type", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("sharing_backend",
				Labels("common-backend"),
				Expr("command", `["whatever"]`),
				Str("filename", "something.tf"),
				Expr("type", `tofu`),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, `unrecognized sharing backend type: tofu`))
	})

	t.Run("sharing_backend with invalid command element", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("sharing_backend",
				Labels("common-backend"),
				Expr("command", `["whatever", false, "else"]`),
				Str("filename", "something.tf"),
				Expr("type", `terraform`),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, `element 1 of attribute sharing_backend.command is not a string but bool`))
	})

	t.Run("basic working sharing_backend", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("sharing_backend",
				Labels("common-backend"),
				Expr("command", `["whatever"]`),
				Str("filename", "something.tf"),
				Expr("type", "terraform"),
			).String(),
		})
		cfg, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		assert.NoError(t, err)
		test.AssertTerramateConfig(t, cfg, hcl.Config{
			Terramate: expectedRootTerramate(),
			SharingBackends: hcl.SharingBackends{
				{
					Name:     "common-backend",
					Filename: "something.tf",
					Type:     hcl.TerraformSharingBackend,
					Command:  []string{"whatever"},
				},
			},
		})
	})
}

func TestParserSharingInput(t *testing.T) {
	t.Run("input with no label", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Doc(
				Block("input",
					Expr("backend", "something"),
					Expr("value", `outputs.something`),
					Str("from_stack_id", "other-stack"),
				),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, "expected a single label but 0 given"))
	})

	t.Run("input with more than 1 label", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("input",
				Labels("label_1", "label_2"),
				Expr("backend", "something"),
				Expr("value", `outputs.something`),
				Str("from_stack_id", "other-stack"),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, "expected a single label but 2 given"))
	})

	t.Run("input with no backend", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("input",
				Labels("var_name"),
				Expr("value", `outputs.someval`),
				Str("from_stack_id", "somestack"),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, `attribute "input.backend" is required`))
	})

	t.Run("input with no value", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("input",
				Labels("var_name"),
				Str("backend", "some-backend"),
				Str("from_stack_id", "somestack"),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, `attribute "input.value" is required`))
	})

	t.Run("input with no from_stack_id", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("input",
				Labels("var_name"),
				Str("backend", "some-backend"),
				Expr("value", `outputs.someval`),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, `attribute "input.from_stack_id" is required`))
	})

	t.Run("basic working inputs", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Doc(
				Block("input",
					Labels("var_name"),
					Str("backend", "some-backend"),
					Expr("value", `outputs.someval`),
					Str("from_stack_id", "somestack"),
				),

				Block("input",
					Labels("var_name_2"),
					Str("backend", "some-backend"),
					Expr("value", `outputs.someval2`),
					Str("from_stack_id", "somestack"),
				),
			).String(),
		})
		cfg, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		assert.NoError(t, err)
		assert.EqualInts(t, 2, len(cfg.Inputs))
		// cfg.Inputs will be validated later when evaluated in the config/input.go
	})
}

func TestParserSharingOutput(t *testing.T) {
	t.Run("output with no label", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Doc(
				Block("output",
					Expr("backend", "something"),
					Expr("value", `outputs.something`),
				),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, "expected a single label but 0 given"))
	})

	t.Run("output with more than 1 label", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("output",
				Labels("label_1", "label_2"),
				Expr("backend", "something"),
				Expr("value", `outputs.something`),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, "expected a single label but 2 given"))
	})

	t.Run("input with no backend", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("output",
				Labels("var_name"),
				Expr("value", `outputs.someval`),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, `attribute "input.backend" is required`))
	})

	t.Run("input with no value", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Block("output",
				Labels("var_name"),
				Str("backend", "some-backend"),
			).String(),
		})
		_, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		errtest.Assert(t, err, errors.E(hcl.ErrTerramateSchema, `attribute "input.value" is required`))
	})

	t.Run("basic working outputs", func(t *testing.T) {
		s := sandbox.NoGit(t, true)
		s.BuildTree([]string{
			`f:cfg.tm:` + Doc(
				Block("output",
					Labels("var_name"),
					Str("backend", "some-backend"),
					Expr("value", `outputs.someval`),
				),

				Block("output",
					Labels("var_name_2"),
					Str("backend", "some-backend"),
					Expr("value", `outputs.someval2`),
				),
			).String(),
		})
		cfg, err := hcl.ParseDir(s.RootDir(), s.RootDir(), hcl.SharingIsCaringExperimentName)
		assert.NoError(t, err)
		assert.EqualInts(t, 2, len(cfg.Outputs))
		// cfg.Outputs will be validated later when evaluated in the config/input.go
	})
}

// this comes from using sandbox.NoGit(t, true)
func expectedRootTerramate() *hcl.Terramate {
	return &hcl.Terramate{
		RequiredVersion:                 "> 0.0.1",
		RequiredVersionAllowPreReleases: true,
	}
}
