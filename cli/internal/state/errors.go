package state

import (
	"errors"
	"fmt"
)

// Error classes for caller-facing failures. They let the board's HTTP write
// endpoints map a Store error to a 4xx status without parsing messages: match with
// errors.Is(err, ErrInvalidInput) / errors.Is(err, ErrConflict). A missing spec or
// epic is reported as a wrapped fs.ErrNotExist (see ReadSpec / GetEpic).
var (
	// ErrInvalidInput marks input the caller must fix (bad id, color, title,
	// unknown epic reference…).
	ErrInvalidInput = errors.New("invalid input")
	// ErrConflict marks a request that is well-formed but clashes with current
	// state (epic already exists, epic still referenced, focusing a closed spec…).
	ErrConflict = errors.New("conflict")
)

// classifiedError keeps the human message unchanged (no "invalid input: " prefix,
// so CLI output stays readable) while matching its class through errors.Is.
type classifiedError struct {
	class   error
	message string
}

func (e classifiedError) Error() string        { return e.message }
func (e classifiedError) Is(target error) bool { return target == e.class }

func invalidInputf(format string, args ...any) error {
	return classifiedError{class: ErrInvalidInput, message: fmt.Sprintf(format, args...)}
}

func conflictf(format string, args ...any) error {
	return classifiedError{class: ErrConflict, message: fmt.Sprintf(format, args...)}
}
