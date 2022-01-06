package cli_test

import (
	"log"
	"os"
	"testing"
)

// The TestMain function creates a terramate command for testing purposes and
// deletes it after the tests have been run.
func TestMain(m *testing.M) {
	topTmpdir, err := os.MkdirTemp("", "cmd-terramate-test-")
	if err != nil {
		log.Fatal(err)
	}
}
