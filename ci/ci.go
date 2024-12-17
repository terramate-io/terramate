// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ci

import (
	"os"
	"strings"
)

// PlatformType is the CI/CD platform.
type PlatformType int

// List of supported CI/CD platforms.
const (
	// PlatformLocal represents the local user environment.
	PlatformLocal PlatformType = iota
	PlatformGithub
	PlatformGitlab
	PlatformGenericCI
	PlatformAppVeyor
	PlatformAzureDevops
	PlatformBamboo
	PlatformBitBucket
	PlatformBuddyWorks
	PlatformBuildKite
	PlatformCircleCI
	PlatformCirrus
	PlatformCodeBuild
	PlatformHeroku
	PlatformHudson
	PlatformJenkins
	PlatformMyGet
	PlatformTeamCity
	PlatformTravis
)

// DetectPlatformFromEnv detects PlatformType based on environment variables.
func DetectPlatformFromEnv() PlatformType {
	typ := PlatformLocal

	if isEnvVarSet("GITHUB_ACTIONS") {
		typ = PlatformGithub
	} else if isEnvVarSet("GITLAB_CI") {
		typ = PlatformGitlab
	} else if isEnvVarSet("BITBUCKET_BUILD_NUMBER") {
		typ = PlatformBitBucket
	} else if isEnvVarSet("TF_BUILD") {
		typ = PlatformAzureDevops
	} else if isEnvVarSet("APPVEYOR") {
		typ = PlatformAppVeyor
	} else if isEnvVarSet("bamboo.buildKey") {
		typ = PlatformBamboo
	} else if isEnvVarSet("BUDDY") {
		typ = PlatformBuddyWorks
	} else if isEnvVarSet("BUILDKITE") {
		typ = PlatformBuildKite
	} else if isEnvVarSet("CIRCLECI") {
		typ = PlatformCircleCI
	} else if isEnvVarSet("CIRRUS_CI") {
		typ = PlatformCirrus
	} else if isEnvVarSet("CODEBUILD_CI") {
		typ = PlatformCodeBuild
	} else if isEnvVarSet("HEROKU_TEST_RUN_ID") {
		typ = PlatformHeroku
	} else if strings.HasPrefix(os.Getenv("BUILD_TAG"), "hudson-") {
		typ = PlatformHudson
	} else if isEnvVarSet("JENKINS_URL") {
		typ = PlatformJenkins
	} else if os.Getenv("BuildRunner") == "MyGet" {
		typ = PlatformMyGet
	} else if isEnvVarSet("TEAMCITY_VERSION") {
		typ = PlatformTeamCity
	} else if isEnvVarSet("TRAVIS") {
		typ = PlatformTravis
	} else if isEnvVarSet("CI") {
		typ = PlatformGenericCI
	}
	return typ
}

func isEnvVarSet(key string) bool {
	val := os.Getenv(key)
	return val != "" && val != "0" && val != "false"
}
