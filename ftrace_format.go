package obs

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type fieldFlag int

const (
	fieldFlagArray fieldFlag = 1 << iota
	fieldFlagPointer
	fieldFlagSigned
	fieldFlagString
	fieldFlagDynamic // __data_loc
	fieldFlagLong
	fieldFlagFlag
	fieldFlagSymbolic
)

// field describes one field associated with a ftrace event.
//
// For tracepoints, $debugfs/tracing/events/*/*/format holds the field
// description for each event.
type field struct {
	name   string
	offset int
	size   int
	flags  fieldFlag
	signed bool
}

// format is the metadata associated with a ftrace event
type format struct {
	fields []field
}

// State of the format description parser.
// Start -> InFormat -> CommonFields -> Fields -> End
type formatParseState int

const (
	stateStart formatParseState = iota
	stateInFormat
	stateCommonFields
	stateFields
	stateEnd
)

type formatParseContext struct {
	state formatParseState
}

type tokenType int

const (
	tokenTypeNone tokenType = iota
	tokenTypeError
	tokenTypeSpace
	tokenTypeNewline
	tokenTypeOperator
	tokenTypeDelimiter
	tokenTypeIdentifier
)

func isSpace(c byte) bool {
	return c == ' ' || c-'\t' < 5
}

func isAlpha(c byte) bool {
	return (c|32)-'a' < 26
}

func isDigit(c byte) bool {
	return (c - '0') < 10
}

func isAlphaNum(c byte) bool {
	return isAlpha(c) || isDigit(c)
}

func isPrint(c byte) bool {
	return c-0x20 < 0x5f
}

func getType(c byte) tokenType {
	if c == '\n' {
		return tokenTypeNewline
	}
	if isSpace(c) {
		return tokenTypeSpace
	}
	if isAlphaNum(c) || c == '_' {
		return tokenTypeIdentifier
	}
	if !isPrint(c) {
		return tokenTypeError
	}
	if c == '(' || c == ')' || c == ',' {
		return tokenTypeDelimiter
	}

	return tokenTypeOperator
}

type tokenCtx struct {
	input []byte
	index int
	token []byte
}

func (ctx *tokenCtx) init(str string) {
	ctx.input = []byte(str)
	ctx.token = make([]byte, 0, 32)
}

func (ctx *tokenCtx) peekChar() byte {
	if ctx.index > (len(ctx.input) - 1) {
		return 0
	}
	return ctx.input[ctx.index]
}

func (ctx *tokenCtx) getChar() byte {
	c := ctx.peekChar()
	ctx.index++
	ctx.token = append(ctx.token, c)
	return c
}

// extend appends consecutive characters with the specified type to the current
// ctx.token.
func (ctx *tokenCtx) extend(t tokenType) {
	for c := ctx.peekChar(); getType(c) == t; {
		ctx.index++
		ctx.token = append(ctx.token, c)
		c = ctx.peekChar()
	}
}

// discard consumes characters until 'end' is found.
func (ctx *tokenCtx) discard(end byte) bool {
	var c byte

	for c = ctx.peekChar(); c != 0 && c != end; {
		ctx.index++
		c = ctx.peekChar()
	}
	if c == 0 {
		return false
	}
	// consume 'end'
	ctx.index++
	return true
}

func (ctx *tokenCtx) getToken() (string, tokenType) {
next:
	ctx.token = make([]byte, 0, 32)

	c := ctx.getChar()
	if c == 0 {
		return "", tokenTypeNone
	}

	t := getType(c)
	switch t {
	case tokenTypeIdentifier:
		ctx.extend(t)
		return string(ctx.token), t
	case tokenTypeSpace:
		ctx.extend(t)
		goto next
	case tokenTypeNewline:
		return "", tokenTypeNone
	case tokenTypeOperator:
		return string(ctx.token), t
	case tokenTypeDelimiter:
		fallthrough
	default:
		return "", tokenTypeError
	}

}

func parseFieldTypeName(str string, out *field) error {
	if !strings.HasPrefix(str, "field:") {
		return errors.New("format: expected 'field:'")
	}

	str = str[len("field:"):]
	ctx := tokenCtx{}
	ctx.init(str)

	for token, t := ctx.getToken(); token != ""; {
		if t == tokenTypeError {
			return errors.New("format: error parsing field: " + str)
		}

		if t == tokenTypeOperator {
			switch token {
			// Note that the field is an array and discard its size (the size property
			// will tell us anyway)
			case "[":
				out.flags |= fieldFlagArray
				if !ctx.discard(']') {
					return fmt.Errorf("format: unmatched '[' in \"%s\"", str)
				}
			}
		} else if t == tokenTypeIdentifier {
			switch token {
			case "__data_loc":
				out.flags |= fieldFlagDynamic
			default:
				// The last identifier is the variable name.
				out.name = token
			}
		}

		token, t = ctx.getToken()
	}

	return nil
}

func parseFieldNumber(str string, prefix string, out *int) error {
	if !strings.HasPrefix(str, prefix) {
		return errors.New("format: expected '" + prefix + "'")
	}

	i, err := strconv.Atoi(str[len(prefix):])
	if err != nil {
		return err
	}

	*out = i

	return nil
}

func parseFieldOffset(str string, out *field) error {
	return parseFieldNumber(str, "offset:", &out.offset)
}

func parseFieldSize(str string, out *field) error {
	return parseFieldNumber(str, "size:", &out.size)
}

func parseFieldSign(str string, out *field) error {
	var s int

	if err := parseFieldNumber(str, "signed:", &s); err != nil {
		return err
	}

	out.signed = s == 1

	return nil
}

func parseField(field string, out *field) error {
	parts := strings.Split(field, ";")

	// 3 properties are mandatory:
	//  - field: $c_type $name;
	//  - offset: $number;
	//  - size: $number;
	// The 'signed' property came later.
	if len(parts) < 3 {
		return fmt.Errorf("format: unexpected number of field properties (%d) in %s",
			len(parts), field)
	}

	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	if err := parseFieldTypeName(parts[0], out); err != nil {
		return err
	}
	if err := parseFieldOffset(parts[1], out); err != nil {
		return err
	}
	if err := parseFieldSize(parts[2], out); err != nil {
		return err
	}

	if len(parts) == 3 {
		return nil
	}

	if err := parseFieldSign(parts[3], out); err != nil {
		return err
	}

	return nil
}

// initFromReader parses a ftrace description. This looks like:
//
//   name: sched_process_fork
//   ID: 267
//   format:
//   	field:unsigned short common_type;	offset:0;	size:2;	signed:0;
//   	field:unsigned char common_flags;	offset:2;	size:1;	signed:0;
//   	field:unsigned char common_preempt_count;	offset:3;	size:1;	signed:0;
//   	field:int common_pid;	offset:4;	size:4;	signed:1;
//
//   	field:char parent_comm[16];	offset:8;	size:16;	signed:1;
//   	field:pid_t parent_pid;	offset:24;	size:4;	signed:1;
//   	field:char child_comm[16];	offset:28;	size:16;	signed:1;
//   	field:pid_t child_pid;	offset:44;	size:4;	signed:1;
//
//   print fmt: "comm=%s pid=%d child_comm=%s child_pid=%d", REC->parent_comm, REC->parent_pid, REC->child_comm, REC->child_pid
func (f *format) initFromReader(r io.Reader) error {
	ctx := formatParseContext{
		state: stateStart,
	}
	scanner := bufio.NewScanner(r)

	for scanner.Scan() && ctx.state != stateEnd {
		if err := scanner.Err(); err != nil {
			return err
		}

		line := scanner.Text()

		// Scan for /^format:\n$/.
		if line == "format:" {
			if ctx.state != stateStart {
				return errors.New("format: unexpected format marker")
			}
			ctx.state = stateCommonFields
			continue
		}

		// A new line:
		//  - separates common fields to per-event fields and
		//  - signals the end of the format section
		if line == "" {
			switch ctx.state {
			case stateCommonFields:
				ctx.state = stateFields
				continue
			case stateFields:
				ctx.state = stateEnd
				continue
			}
		}

		// Parse a field.
		if ctx.state == stateCommonFields || ctx.state == stateFields {
			field := field{}
			if err := parseField(line, &field); err != nil {
				return err
			}
			f.fields = append(f.fields, field)
		}
	}

	// We couldn't parse any field :/
	if len(f.fields) == 0 {
		return errors.New("format: no field found")
	}

	return nil
}

func (f *format) initFromFile(filename string) error {
	fp, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fp.Close()
	return f.initFromReader(fp)
}

func (f *format) findField(name string) *field {
	for i := range f.fields {
		if f.fields[i].name == name {
			return &f.fields[i]
		}
	}

	return nil
}

// XXX: if we ever need to support bigEndian machines, this won't be true!
var nativeEndian = binary.LittleEndian

func decodeIntInternal(data []byte, field *field) (int, error) {
	switch field.size {
	case 1:
		if field.signed {
			return int(int8(data[field.offset])), nil
		}
		return int(data[field.offset]), nil
	case 2:
		v := nativeEndian.Uint16(data[field.offset : field.offset+2])
		if field.signed {
			return int(int16(v)), nil
		}
		return int(v), nil
	case 4:
		v := nativeEndian.Uint32(data[field.offset : field.offset+4])
		if field.signed {
			return int(int32(v)), nil
		}
		return int(v), nil
	case 8:
		v := nativeEndian.Uint64(data[field.offset : field.offset+8])
		return int(v), nil
	default:
		return 0, fmt.Errorf("unexpected field size: %d", field.size)
	}
}

func (f *format) decodeInt(data []byte, name string) (int, error) {
	field := f.findField(name)
	if field == nil {
		return 0, fmt.Errorf("no field named '%s'", name)
	}

	return decodeIntInternal(data, field)
}
