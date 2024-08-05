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

	// this comes from using sandbox.NoGit(t, true)
	expectedRootTerramate := &hcl.Terramate{
		RequiredVersion:                 "> 0.0.1",
		RequiredVersionAllowPreReleases: true,
	}

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
			Terramate: expectedRootTerramate,
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
