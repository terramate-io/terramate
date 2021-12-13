package cli_test

import "testing"

func TestStackMetadata(t *testing.T) {

	type testcase struct {
		name   string
		layout []string
	}

	tcases := []testcase{
		{
			name:   "no stacks",
			layout: []string{},
		},
		{
			name:   "single stacks",
			layout: []string{"s:stack"},
		},
		{
			name: "two stacks",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
			},
		},
		{
			name: "three stacks and some non-stack dirs",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
				"s:stack-3",
				"d:non-stack",
				"d:non-stack-2",
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			t.Skip("TODO")
		})
	}
}
