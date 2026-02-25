package app

import "errors"

type codedError struct {
	code int
	err  error
}

func (e codedError) Error() string {
	return e.err.Error()
}

func (e codedError) Unwrap() error {
	return e.err
}

func WithCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return codedError{code: code, err: err}
}

func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var ce codedError
	if errors.As(err, &ce) {
		return ce.code
	}
	return ExitSystem
}
