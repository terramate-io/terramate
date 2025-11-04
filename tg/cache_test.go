// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/tg"
)

type Exister interface {
	FileExists(p string) bool
}

type statExists struct {
	statCount int64
}

func newStatExists() *statExists {
	return &statExists{}
}

func (s *statExists) Exists(p string) bool {
	s.statCount++
	_, err := os.Stat(p)
	return err == nil
}

func (s *statExists) FileExists(p string) bool {
	s.statCount++
	_, err := os.Stat(p)
	return err == nil
}

func FindInParent(t *testing.T, dir, name string, exists Exister) {
	dir = filepath.Dir(dir)

	for range 5 {
		if exists.FileExists(filepath.Join(dir, name)) {
			return
		}

		dir = filepath.Dir(dir)
	}

	t.Fatalf("not found: %s", filepath.Join(dir, name))
}

const (
	I = 10
	J = 10
	K = 1000
)

func setupFiles(t *testing.T) sandbox.S {
	s := sandbox.NewWithGitConfig(t, sandbox.GitConfig{
		LocalBranchName:         "main",
		DefaultRemoteName:       "origin",
		DefaultRemoteBranchName: "default",
	})

	root1 := s.RootDir()

	test.WriteFile(t, root1, "level1.hcl", "")
	for i := range I {
		root2 := fmt.Sprintf("%s/%d", root1, i)

		test.MkdirAll(t, root2)
		test.WriteFile(t, root2, "level2.hcl", "")

		for j := range J {
			root3 := fmt.Sprintf("%s/%d", root2, j)

			test.MkdirAll(t, root3)
			test.WriteFile(t, root3, "level3.hcl", "")

			for k := range K {
				root4 := fmt.Sprintf("%s/%d", root3, k)
				test.MkdirAll(t, root4)

				test.WriteFile(t, root4, fmt.Sprintf("file.%d.%d.%d.hcl", i, j, k), "")
			}
		}
	}

	s.Git().CommitAll("create repo")
	s.Git().Push("main")

	return s
}

func TestPerformance1(t *testing.T) {
	s := setupFiles(t)
	root := s.RootDir()

	statExists := newStatExists()
	callCount := 0

	testStart := time.Now()
	for i := range I {
		for j := range J {
			for k := range K {
				dir := fmt.Sprintf("%s/%d/%d/%d/", root, i, j, k)
				FindInParent(t, dir, "level1.hcl", statExists)
				FindInParent(t, dir, "level2.hcl", statExists)
				FindInParent(t, dir, "level3.hcl", statExists)
				callCount += 3
			}
		}
	}
	testElapsed := time.Since(testStart)

	fmt.Printf("Stat calls: %d\n", statExists.statCount)
	fmt.Printf("Test time: %s\n", testElapsed)

	assert.EqualInts(t, I*J*K*3, callCount)

	t.FailNow()
}

func TestPerformance2(t *testing.T) {
	s := setupFiles(t)
	root := s.RootDir()

	callCount := 0

	setupStart := time.Now()
	cachedExists, err := tg.NewFileExistsCache(s.RootDir(), tg.IsHCLFile)
	setupElapsed := time.Since(setupStart)

	fmt.Printf("Cache setup time: %s\n", setupElapsed)

	assert.NoError(t, err)

	testStart := time.Now()
	for i := range I {
		for j := range J {
			for k := range K {
				dir := fmt.Sprintf("%s/%d/%d/%d/", root, i, j, k)
				FindInParent(t, dir, "level1.hcl", cachedExists)
				FindInParent(t, dir, "level2.hcl", cachedExists)
				FindInParent(t, dir, "level3.hcl", cachedExists)
				callCount += 3
			}
		}
	}
	testElapsed := time.Since(testStart)
	fmt.Printf("Test time: %s\n", testElapsed)

	assert.EqualInts(t, I*J*K*3, callCount)

	//os.Chdir(root)

	//findStart := time.Now()
	//cmd := exec.Command("bash", "-c", "git ls-files >/dev/null")
	//out, err := cmd.CombinedOutput()

	//findElapsed := time.Since(findStart)

	//fmt.Println("find output: " + string(out))

	//assert.NoError(t, err)

	//fmt.Printf("Find time: %s\n", findElapsed)
	t.FailNow()
}
