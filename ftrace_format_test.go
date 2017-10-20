package obs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDigit(t *testing.T) {
	digits := [256]bool{
		'0': true, '1': true, '2': true, '3': true, '4': true, '5': true,
		'6': true, '7': true, '8': true, '9': true,
	}
	for c, expected := range digits {
		assert.Equal(t, expected, isDigit(byte(c)))
	}
}
func TestParseFieldOffset(t *testing.T) {
	tests := []struct {
		input    string
		valid    bool
		expected int
	}{
		{"offset:4", Valid, 4},
		{"offs:4", Invalid, 0},
	}

	for _, test := range tests {
		var f field

		err := parseFieldOffset(test.input, &f)
		if !test.valid {
			assert.NotNil(t, err)
			continue
		}

		assert.Nil(t, err)
		assert.Equal(t, test.expected, f.offset)
	}
}

type tokenOutput struct {
	token string
	kind  tokenType
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		valid    bool
		expected []tokenOutput
	}{
		{
			"pid_t parent_pid", Valid, []tokenOutput{
				{"pid_t", tokenTypeIdentifier},
				{"parent_pid", tokenTypeIdentifier},
			},
		}, {
			"char parent_comm[16]", Valid, []tokenOutput{
				{"char", tokenTypeIdentifier},
				{"parent_comm", tokenTypeIdentifier},
				{"[", tokenTypeOperator},
				{"16", tokenTypeIdentifier},
				{"]", tokenTypeOperator},
			},
		}, {
			"__data_loc char[] filename", Valid, []tokenOutput{
				{"__data_loc", tokenTypeIdentifier},
				{"char", tokenTypeIdentifier},
				{"[", tokenTypeOperator},
				{"]", tokenTypeOperator},
				{"filename", tokenTypeIdentifier},
			},
		},
	}

	for _, test := range tests {
		var got []tokenOutput

		ctx := tokenCtx{}
		ctx.init(test.input)

		for token, kind := ctx.getToken(); token != ""; {
			if test.valid {
				assert.NotEqual(t, kind, tokenTypeError)
			}

			got = append(got, tokenOutput{token, kind})

			token, kind = ctx.getToken()
		}

		assert.Equal(t, test.expected, got)

	}
}

func TestParseFieldTypeName(t *testing.T) {
	tests := []struct {
		input    string
		valid    bool
		expected field
	}{
		{"field:unsigned short common_type", Valid, field{name: "common_type", flags: 0}},
		{"field:__data_loc char[] filename", Valid, field{name: "filename", flags: fieldFlagDynamic | fieldFlagArray}},
	}

	for _, test := range tests {
		var f field

		err := parseFieldTypeName(test.input, &f)
		if !test.valid {
			assert.NotNil(t, err)
			continue
		}

		assert.Nil(t, err)
		assert.Equal(t, test.expected, f)
	}
}

func TestParseField(t *testing.T) {
	tests := []struct {
		input    string
		valid    bool
		expected field
	}{
		{
			"	field:unsigned short common_type;	offset:4;	size:2;	signed:1;", valid,
			field{name: "common_type", offset: 4, size: 2, signed: true},
		},
		{
			"	field:char parent_comm[16];	offset:8;	size:16;	signed:1;", valid,
			field{name: "parent_comm", offset: 8, size: 16, signed: true, flags: fieldFlagArray},
		},
	}

	for _, test := range tests {
		var f field

		err := parseField(test.input, &f)
		if !test.valid {
			assert.NotNil(t, err)
			continue
		}

		assert.Nil(t, err)
		assert.Equal(t, test.expected, f)
	}
}

const forkFormat = `
name: sched_process_fork
ID: 267
format:
	field:unsigned short common_type;	offset:0;	size:2;	signed:0;
	field:unsigned char common_flags;	offset:2;	size:1;	signed:0;
	field:unsigned char common_preempt_count;	offset:3;	size:1;	signed:0;
	field:int common_pid;	offset:4;	size:4;	signed:1;

	field:char parent_comm[16];	offset:8;	size:16;	signed:1;
	field:pid_t parent_pid;	offset:24;	size:4;	signed:1;
	field:char child_comm[16];	offset:28;	size:16;	signed:1;
	field:pid_t child_pid;	offset:44;	size:4;	signed:1;

print fmt: "comm=%s pid=%d child_comm=%s child_pid=%d", REC->parent_comm, REC->parent_pid, REC->child_comm, REC->child_pid`

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input    string
		valid    bool
		expected []field
	}{
		{
			input: forkFormat,
			valid: Valid,
			expected: []field{
				field{name: "common_type", offset: 0, size: 2, signed: false},
				field{name: "common_flags", offset: 2, size: 1, signed: false},
				field{name: "common_preempt_count", offset: 3, size: 1, signed: false},
				field{name: "common_pid", offset: 4, size: 4, signed: true},
				field{name: "parent_comm", offset: 8, size: 16, signed: true, flags: fieldFlagArray},
				field{name: "parent_pid", offset: 24, size: 4, signed: true},
				field{name: "child_comm", offset: 28, size: 16, signed: true, flags: fieldFlagArray},
				field{name: "child_pid", offset: 44, size: 4, signed: true},
			},
		},
	}

	for i := range tests {
		test := &tests[i]
		r := strings.NewReader(test.input)
		f := format{}
		err := f.initFromReader(r)
		if !test.valid {
			assert.NotNil(t, err)
			continue
		}

		assert.Nil(t, err)
		assert.Equal(t, test.expected, f.fields)
	}
}

const execFormat = `
name: sched_process_exec
ID: 266
format:
	field:unsigned short common_type;	offset:0;	size:2;	signed:0;
	field:unsigned char common_flags;	offset:2;	size:1;	signed:0;
	field:unsigned char common_preempt_count;	offset:3;	size:1;	signed:0;
	field:int common_pid;	offset:4;	size:4;	signed:1;

	field:__data_loc char[] filename;	offset:8;	size:4;	signed:1;
	field:pid_t pid;	offset:12;	size:4;	signed:1;
	field:pid_t old_pid;	offset:16;	size:4;	signed:1;

print fmt: "filename=%s pid=%d old_pid=%d", __get_str(filename), REC->pid, REC->old_pid
`

var execData = []byte{
	0x0a, 0x01, 0x00, 0x00, 0xb3, 0x01, 0x00, 0x00, 0x14, 0x00, 0x0a, 0x00, 0xb3, 0x01, 0x00, 0x00,
	0xb3, 0x01, 0x00, 0x00, 0x2f, 0x62, 0x69, 0x6e, 0x2f, 0x62, 0x61, 0x73, 0x68, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
}

const bashCmdline = "/bin/bash"
const bashPID = 435

func TestDecodeInt(t *testing.T) {
	var f format

	f.initFromReader(strings.NewReader(execFormat))
	decoded, err := f.decodeInt(execData, "pid")
	assert.Nil(t, err)
	assert.Equal(t, bashPID, decoded)
}
