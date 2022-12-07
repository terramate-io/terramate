// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package trigger provides functionality that help manipulate stacks triggers.
package trigger

import (
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// Info represents the parsed contents of a trigger
// for triggers created by Terramate.
type Info struct {
	// Ctime is unix timestamp of when the trigger was created.
	Ctime int64
	// Reason is the reason why the trigger was created, if any.
	Reason string
}

const triggersDir = ".tmtriggers"

// Parse will parse the body of a trigger file.
func Parse(body string) (Info, error) {
	return Info{}, nil
}

// Dir will return the triggers directory for the project rooted at rootdir.
// Both rootdir and the returned value are host absolute paths.
func Dir(rootdir string) string {
	return filepath.Join(rootdir, triggersDir)
}

// Create creates a trigger for a stack with the given path and the given reason
// inside the project rootdir.
func Create(rootdir string, path project.Path, reason string) error {

	id, err := uuid.NewRandom()
	if err != nil {
		return errors.E(err, "creating trigger UUID")
	}
	triggerID := id.String()
	triggerDir := filepath.Join(rootdir, triggersDir, path.String())

	if err := os.MkdirAll(triggerDir, 0775); err != nil {
		return errors.E(err, "creating trigger dir")
	}

	ctime := time.Now().Unix()

	gen := hclwrite.NewEmptyFile()
	triggerBody := gen.Body().AppendNewBlock("trigger", nil).Body()
	triggerBody.SetAttributeValue("ctime", cty.NumberIntVal(ctime))
	triggerBody.SetAttributeValue("reason", cty.StringVal(reason))

	triggerPath := filepath.Join(triggerDir, triggerID+".tm.hcl")

	if err := os.WriteFile(triggerPath, gen.Bytes(), 0666); err != nil {
		return errors.E(err, "creating trigger file")
	}

	log.Debug().
		Str("action", "trigger.Create").
		Int64("ctime", ctime).
		Str("reason", reason).
		Msg("trigger file created")

	return nil
}
