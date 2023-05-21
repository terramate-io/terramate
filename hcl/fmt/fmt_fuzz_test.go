//go:build go1.18 && linux

package fmt_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/hcl/fmt"
)

func FuzzFormatMultiline(f *testing.F) {
	seedCorpus := []string{
		"attr",
		"attr.value",
		"attr.*.value",
		"global.str",
		`"a ${global.str}"`,
		`"${global.obj}"`,
		`"${global.list} fail`,
		`{}`,
		`{a=[]}`,
		`[{}]`,
		`[{a=666}]`,
		`[[]]`,
		`10`,
		`"test"`,
		`[1, 2, 3]`,
		`[[1], [2], [3]]`,
		`a()`,
		`föo("föo") + föo`,
		`${var.name}`,
		`{ for k in var.val : k => k }`,
		`[ for k in var.val : k => k ]`,
	}

	for _, seed := range seedCorpus {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, str string) {
		// WHY? because HCL uses the big.Float library for numbers and then
		// fuzzer can generate huge number strings like 100E101000000 that will
		// hang the process and eat all the memory....
		const bigNumRegex = "[\\d]+[\\s]*[.]?[\\s]*[\\d]*[EepP]{1}[\\s]*[+-]?[\\s]*[\\d]+"
		hasBigNumbers, _ := regexp.MatchString(bigNumRegex, str)
		if hasBigNumbers {
			return
		}

		if strings.Contains(str, "/*") || strings.Contains(str, "//") {
			// The formatting tested here does not support comments
			// Since it is used only for generated code (that has no comments)
			return
		}

		const testattr = "attr"

		cfg := testattr + " = " + str

		// WHY: When we try to format "attr = 0.0 .0" it will format to
		// attr = 0.0.0 which then is NOT valid HCL =P.
		// Since hashicorp's hcl.Format is the one doing this we filter
		// out hcl.Format mistakes here to focus on our own mistakes
		// on hcl.FormatMultiline.
		defaultFmt, err := fmt.Format(cfg, "default-fmt.hcl")
		if err != nil || !isValidHCL(defaultFmt) {
			return
		}

		got := formatMultiline(t, cfg)
		assertIsHCL(t, cfg, got)
		reformatted := formatMultiline(t, got)

		if got != reformatted {
			assert.EqualStrings(t, got, reformatted,
				"re-formatting should produce same exact result")
		}
	})
}

func assertIsHCL(t *testing.T, orig, code string) {
	t.Helper()

	parser := hclparse.NewParser()
	_, diags := parser.ParseHCL([]byte(code), "fuzz")
	if diags.HasErrors() {
		t.Fatalf("original code:\n%s\nformatted version:\n%s\nis not valid HCL:%v", orig, code, diags)
	}
}

func isValidHCL(code string) bool {
	parser := hclparse.NewParser()
	_, diags := parser.ParseHCL([]byte(code), "fuzz")
	return !diags.HasErrors()
}

func formatMultiline(t *testing.T, code string) string {
	t.Helper()

	got, err := fmt.FormatMultiline(code, "fuzz.hcl")
	assert.NoError(t, err)

	return got
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
