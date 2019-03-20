package obs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNS(t *testing.T) {
	tests := []struct {
		s        string
		valid    bool
		expected uint64
	}{
		{"cgroup:[4026531835]", true, 4026531835},
	}

	for _, test := range tests {
		ns, err := parseNS(test.s)
		if !test.valid {
			assert.Error(t, err)
			continue
		}
		assert.NoError(t, err)
		assert.Equal(t, test.expected, ns)
	}
}
