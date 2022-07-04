package stack

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/tf"
	"github.com/rs/zerolog/log"
)

func scanTfModules(dir string, visited map[string]struct{}) ([]tf.Module, error) {
	logger := log.With().
		Str("action", "scanTfModules()").
		Str("path", dir).
		Logger()

	if _, ok := visited[dir]; ok {
		return nil, nil
	}

	logger.Debug().Msg("Read dir.")

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.E(err, "listing files of directory %q", dir)
	}

	var allmodules []tf.Module
	for _, file := range files {
		logger = logger.With().
			Str("file", file.Name()).
			Logger()

		if filepath.Ext(file.Name()) != ".tf" {
			logger.Trace().Msg("ignoring")
			continue
		}

		path := filepath.Join(dir, file.Name())
		modules, err := tf.ParseModules(path)
		if err != nil {
			return nil, errors.E(err, "scanning tf files in %q", dir)
		}

		logger.Trace().Msg("parsed successfully.")

		allmodules = append(allmodules, modules...)
	}

	visited[dir] = struct{}{}

	for _, mod := range allmodules {
		logger = logger.With().
			Str("module", mod.Source).
			Logger()

		if !mod.IsLocal() {
			logger.Trace().Msg("ignoring non-local module")
			continue
		}

		logger.Trace().Msg("scanning module's dependencies")

		absSource := filepath.Join(dir, mod.Source)
		modules, err := scanTfModules(absSource, visited)
		if err != nil {
			return nil, errors.E(err, "scanning tf files in %q", absSource)
		}

		allmodules = append(allmodules, modules...)
	}

	logger.Trace().
		Msgf("scanner found %d module dependencies for path %q",
			len(allmodules), dir)

	return allmodules, nil
}

func listTfFiles(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var tffiles []string
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".tf" && strings.HasPrefix(f.Name(), ".") {
			continue
		}

		tffiles = append(tffiles, filepath.Join(dir, f.Name()))
	}
	return tffiles, nil
}
