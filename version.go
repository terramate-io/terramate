package terramate

import (
	_ "embed"
	"fmt"
	"strings"

	tfversion "github.com/hashicorp/go-version"
)

//go:embed VERSION
var version string

var tfversionObj *tfversion.Version

func init() {
	var err error
	tfversionObj, err = tfversion.NewSemver(Version())
	if err != nil {
		msg := fmt.Sprintf(
			"terramate version does not adhere to semver specification: %s",
			err.Error(),
		)
		panic(msg)
	}
}

// Version of terramate.
func Version() string {
	return strings.TrimSpace(version)
}
