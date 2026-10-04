package harness

import (
	"errors"
	"fmt"
)

// Code is a stable machine-readable error category.
type Code string

const (
	CodeNoDeadline       Code = "no_deadline"
	CodeWorldContext     Code = "world_context_required"
	CodeEmptyText        Code = "text_required"
	CodeNilCatalog       Code = "catalog_required"
	CodeEmptyDescription Code = "empty_description"
	CodeUnknownChoice    Code = "unknown_choice"
	CodeInvalidScore     Code = "invalid_score"
	CodeInvalidDraft     Code = "invalid_draft"
	CodeConfig           Code = "config"
	CodeHopLimit         Code = "hop_limit"
	CodeJudge            Code = "judge"
	CodeAuthor           Code = "author"
	CodeGate             Code = "gate"
)

// Error is a typed harness failure.
type Error struct {
	Op   string
	Code Code
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return "harness: <nil>"
	}
	if e.Err != nil {
		return fmt.Sprintf("harness.%s: %s: %v", e.Op, e.Msg, e.Err)
	}
	return fmt.Sprintf("harness.%s: %s", e.Op, e.Msg)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// CodeOf returns the harness code when err wraps *Error.
func CodeOf(err error) Code {
	var he *Error
	if errors.As(err, &he) && he != nil {
		return he.Code
	}
	return ""
}

func newErr(op string, code Code, msg string, err error) error {
	return &Error{Op: op, Code: code, Msg: msg, Err: err}
}
