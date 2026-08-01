package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hashicorp/hcl"
	hclparser "github.com/hashicorp/hcl/hcl/parser"

	"vault-manager/internal/domain"
)

var positionPattern = regexp.MustCompile(`^(?:At )?([0-9]+):([0-9]+):?\s*(.*)$`)

type FormatError struct {
	Line    int
	Column  int
	Message string
}

func (e *FormatError) Error() string {
	if e.Line > 0 && e.Column > 0 {
		return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Message)
	}
	return e.Message
}

func Field(format domain.FieldFormat, value string) error {
	if format == domain.FormatNone || strings.TrimSpace(value) == "" {
		return nil
	}
	switch format {
	case domain.FormatJSONObject:
		var object map[string]interface{}
		if err := json.Unmarshal([]byte(value), &object); err != nil {
			return jsonError(value, err)
		}
		if object == nil {
			return &FormatError{Line: 1, Column: 1, Message: "value must be a JSON object"}
		}
	case domain.FormatStringMap:
		var object map[string]string
		if err := json.Unmarshal([]byte(value), &object); err != nil {
			return jsonError(value, err)
		}
		if object == nil {
			return &FormatError{Line: 1, Column: 1, Message: "value must be a JSON object with string values"}
		}
	case domain.FormatPolicy:
		if _, err := hcl.Parse(value); err != nil {
			return hclError(err)
		}
	default:
		return fmt.Errorf("unknown field format %q", format)
	}
	return nil
}

func jsonError(value string, err error) error {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		line, column := offsetPosition(value, syntax.Offset)
		return &FormatError{Line: line, Column: column, Message: syntax.Error()}
	}
	var kind *json.UnmarshalTypeError
	if errors.As(err, &kind) {
		line, column := offsetPosition(value, kind.Offset)
		return &FormatError{Line: line, Column: column, Message: kind.Error()}
	}
	return &FormatError{Message: err.Error()}
}

func hclError(err error) error {
	var positioned *hclparser.PosError
	if errors.As(err, &positioned) {
		return &FormatError{
			Line:    positioned.Pos.Line,
			Column:  positioned.Pos.Column,
			Message: positioned.Err.Error(),
		}
	}
	matches := positionPattern.FindStringSubmatch(err.Error())
	if len(matches) == 4 {
		line, _ := strconv.Atoi(matches[1])
		column, _ := strconv.Atoi(matches[2])
		message := strings.TrimSpace(matches[3])
		if message == "" {
			message = "invalid policy syntax"
		}
		return &FormatError{Line: line, Column: column, Message: message}
	}
	return &FormatError{Message: err.Error()}
}

func offsetPosition(value string, offset int64) (int, int) {
	if offset < 1 {
		return 1, 1
	}
	index := int(offset - 1)
	if index > len(value) {
		index = len(value)
	}
	prefix := []byte(value[:index])
	line := bytes.Count(prefix, []byte{'\n'}) + 1
	lastNewline := bytes.LastIndexByte(prefix, '\n')
	columnBytes := prefix
	if lastNewline >= 0 {
		columnBytes = prefix[lastNewline+1:]
	}
	return line, utf8.RuneCount(columnBytes) + 1
}
