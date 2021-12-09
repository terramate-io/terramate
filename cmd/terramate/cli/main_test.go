package cli_test

import (
	"os"
	"testing"

	"github.com/mineiros-io/terramate"
)

const TerramateTestVersion = "0.0.1-tests"

func TestMain(m *testing.M) {
	// We could use ldflags on go test, but then just running
	// go test ./... Would not work anymore
	// Here we initialize terramate version for testing purposes
	terramate.Version = TerramateTestVersion
	os.Exit(m.Run())
}
