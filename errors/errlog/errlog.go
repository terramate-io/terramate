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

// Package errlog provides functions to log Terramate errors nicely and
// in a consistent manner.
package errlog

import (
	"github.com/mineiros-io/terramate/errors"
	"github.com/rs/zerolog"
)

// Fatal logs the error as a Fatal if the error is not nil.
// If the error is nil this is a no-op.
func Fatal(logger zerolog.Logger, msg string, err error) {
	if err == nil {
		return
	}

	var list *errors.List

	// TODO(katcipis): improve how individual errors.E are logged.
	if errors.As(err, &list) {
		errs := list.Errors()
		for _, err := range errs {
			logger.Error().Msg(err.Error())
		}
	} else {
		logger.Error().Msg(err.Error())
	}

	logger.Fatal().Msg(msg)
}
