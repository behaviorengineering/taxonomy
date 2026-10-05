package catalog

import (
	"errors"
	"fmt"
)

// Code is a stable machine-readable error category.
type Code string

const (
	CodeDuplicateID    Code = "duplicate_id"
	CodeMissingParent  Code = "missing_parent"
	CodeCycle          Code = "cycle"
	CodeAliasCollision Code = "alias_collision"
	CodeInvalidVocab   Code = "invalid_vocab"
	CodeParse          Code = "parse_error"
	CodeMergeCollision Code = "merge_collision"
	CodeInvalidPattern Code = "invalid_pattern"
)

// Error is a typed catalog failure.
type Error struct {
	Op   string
	Code Code
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return "catalog: <nil>"
	}
	if e.Err != nil {
		return fmt.Sprintf("catalog.%s: %s: %v", e.Op, e.Msg, e.Err)
	}
	return fmt.Sprintf("catalog.%s: %s", e.Op, e.Msg)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// CodeOf returns the catalog code when err wraps *Error.
func CodeOf(err error) Code {
	var ce *Error
	if errors.As(err, &ce) && ce != nil {
		return ce.Code
	}
	return ""
}

func newErr(op string, code Code, msg string, err error) error {
	return &Error{Op: op, Code: code, Msg: msg, Err: err}
}
