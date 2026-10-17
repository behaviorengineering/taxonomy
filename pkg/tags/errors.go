package tags

import (
	"errors"
	"fmt"
)

// Code is a stable machine-readable error category.
type Code string

const (
	CodeConfig     Code = "config"
	CodeInvalid    Code = "invalid"
	CodeFailed     Code = "failed"
	CodeNoDeadline Code = "no_deadline"
	CodeJudge      Code = "judge"
	CodeGate       Code = "gate"
)

// ErrTokenInvalid is the validate message when a tag token fails the gate.
const ErrTokenInvalid = "essence tag invalid"

// Error is a typed tags failure.
type Error struct {
	Op   string
	Code Code
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return "tags: <nil>"
	}
	if e.Err != nil {
		return fmt.Sprintf("tags.%s: %s: %v", e.Op, e.Msg, e.Err)
	}
	return fmt.Sprintf("tags.%s: %s", e.Op, e.Msg)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// CodeOf returns the tags code when err wraps *Error.
func CodeOf(err error) Code {
	var te *Error
	if errors.As(err, &te) && te != nil {
		return te.Code
	}
	return ""
}

func newErr(op string, code Code, msg string, err error) error {
	return &Error{Op: op, Code: code, Msg: msg, Err: err}
}
