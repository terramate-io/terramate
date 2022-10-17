package fs

import (
	"os"
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/rs/zerolog/log"
)

// ListTerramateFiles returns a list of terramate related files from the
// directory dir.
func ListTerramateFiles(dir string) ([]string, error) {
	logger := log.With().
		Str("action", "fs.listTerramateFiles()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("listing files")

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.E(err, "reading dir to list Terramate files")
	}

	logger.Trace().Msg("looking for Terramate files")

	files := []string{}

	for _, dirEntry := range dirEntries {
		logger := logger.With().
			Str("entryName", dirEntry.Name()).
			Logger()

		if strings.HasPrefix(dirEntry.Name(), ".") {
			logger.Trace().Msg("ignoring dotfile")
			continue
		}

		if dirEntry.IsDir() {
			logger.Trace().Msg("ignoring dir")
			continue
		}

		filename := dirEntry.Name()
		if isTerramateFile(filename) {
			logger.Trace().Msg("Found Terramate file")
			files = append(files, filename)
		}
	}

	return files, nil
}

// ListTerramateDirs lists Terramate dirs, which are any dirs
// except ones starting with ".".
func ListTerramateDirs(dir string) ([]string, error) {
	logger := log.With().
		Str("action", "fs.ListTerramateDirs()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("listing dirs")

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.E(err, "reading dir to list Terramate dirs")
	}

	logger.Trace().Msg("looking for Terramate directories")

	dirs := []string{}

	for _, dirEntry := range dirEntries {
		logger := logger.With().
			Str("entryName", dirEntry.Name()).
			Logger()

		if !dirEntry.IsDir() {
			logger.Trace().Msg("ignoring non-dir")
			continue
		}

		if strings.HasPrefix(dirEntry.Name(), ".") {
			logger.Trace().Msg("ignoring dotdir")
			continue
		}

		dirs = append(dirs, dirEntry.Name())
	}

	return dirs, nil
}

func isTerramateFile(filename string) bool {
	return strings.HasSuffix(filename, ".tm") || strings.HasSuffix(filename, ".tm.hcl")
}
