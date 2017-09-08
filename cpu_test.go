package obs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	valid   = true
	invalid = false
)

func TestParseOnlineCPUs(t *testing.T) {
	tests := []struct {
		input  string
		valid  bool
		golden []int
	}{
		{"0", valid, []int{0}},
		{"0-3", valid, []int{0, 1, 2, 3}},
		{"2,125-127,128-130", valid, []int{2, 125, 126, 127, 128, 129, 130}},

		{"", invalid, nil},
		{"3-", invalid, nil},
	}

	for i := range tests {
		test := &tests[i]
		output, err := parseOnlineCPUs(test.input)
		assert.Equal(t, test.valid, err == nil)
		assert.Equal(t, test.golden, output)
	}
}
